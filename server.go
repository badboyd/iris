package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/labstack/echo"
)

var (
	idGen      *idGenerator
	uploadPool *bufPool
	saltKey    []byte
	secretKey  []byte

	e = echo.New()

	defaultOption *processingOptions

	errImageMissing                = errors.New("Vui lòng chọn một hình")
	errSourceImageTypeNotSupported = errors.New("Hình bạn đăng có thể không phải định dạng jpg, png, gif hoặc bmp")
	errSourceFileTooBig            = errors.New("Hình bạn đăng có dung lượng quá lớn. Vui lòng đăng hình dưới 10MB")
	errSourceDimensionsTooSmall    = errors.New("Kích thước hình quá nhỏ. Vui lòng đăng hình có kích thước từ 240*240 trở lên")
	errSourceDimensionsTooBig      = errors.New("Kích thước hình quá lớn. Vui lòng đăng hình có kích thước từ 10000*10000 trở xuống")

	imageFileKey              = "imageFile"
	imageFileSizeKey          = "imageSize"
	imageDataKey              = "imageData"
	imageDataBufferKey        = "imageBuffer"
	imageWidthKey             = "width"
	imageHeightKey            = "height"
	imageTypeKey              = "imageType"
	imageProcessingOptionsKey = "processingOptions"
	imageIDKey                = "imageID"
	objectIDKey               = "objectID"
	imageStorageURLKey        = "imageURL"
	startTimeKey              = "startTime"
)

type ctxKey string

// start Iris
func start() {
	// init the handlers first
	// then start the server
	initVips()
	initStorage()

	go func() {
		var logMemStats = config.Iris.LogMemStats

		for range time.Tick(config.Iris.FreeMemoryInterval) {
			debug.FreeOSMemory()

			if logMemStats {
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				log.Infof("[MEMORY USAGE] Sys: %d; HeapIdle: %d; HeapInuse: %d", m.Sys/1024/1024, m.HeapIdle/1024/1024, m.HeapInuse/1024/1024)
			}
		}
	}()

	defaultOption = &processingOptions{
		Dpr:        1,
		Blur:       0,
		Sharpen:    0,
		Enlarge:    false,
		Expand:     false,
		Resize:     resizeFit,
		Quality:    config.Image.Quality,
		Width:      config.Image.Width,
		Height:     config.Image.Height,
		Format:     imageTypes[config.Image.Type],
		Gravity:    gravityOptions{Type: gravityCenter},
		Background: rgbColor{255, 255, 255},
		Watermark:  watermarkOptions{Opacity: 1, Replicate: false, Gravity: gravityCenter},
	}
	log.Debugf("Default image processing config: %+v\n", *defaultOption)

	uploadPool = newBufPool("upload", config.Iris.Concurrency, config.Iris.BufferSize)

	tmp, err := newIDGenerator()
	if err != nil {
		log.Panic("Cannot init id generator: ", err.Error())
	}
	idGen = tmp

	// init salt key if any
	salt := os.Getenv("ONEIMAGE__SALT_KEY")
	secret := os.Getenv("ONEIMAGE__SECRET_KEY")

	if salt != "" && secret != "" {
		if saltKey, err = hex.DecodeString(salt); err != nil {
			log.Panic("Cannot init salt key: ", salt, err.Error())
		}
		if secretKey, err = hex.DecodeString(secret); err != nil {
			log.Panic("Cannot init secret key: ", secret, err.Error())
		}
	}

	e.HTTPErrorHandler = errorsHandler

	if config.PProf.Enabled {
		initPProf()
	}

	if config.Prometheus.Enabled {
		initPrometheus()
	}

	// dummy health check
	e.GET("/health", health)

	// apis group
	// 1i: one image
	// add prometheus
	apiGroup := e.Group("/v1/1i", writePrometheusResponseTime)

	apiGroup.DELETE("/:id", delete, genObjectURL)                                                           //delete image
	apiGroup.PUT("/:id", upload, getAndCheckFileSize, checkTypeAndDimensions, process, genID, genObjectURL) // upload image
	apiGroup.POST("", upload, getAndCheckFileSize, checkTypeAndDimensions, process, genID, genObjectURL)    // upload image

	go startServer()
	waitForInterruptSignal()
}

func writePrometheusResponseTime(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if prometheusEnabled {
			prometheusRequestsTotal.Inc()
			defer startPrometheusDuration(prometheusRequestDuration)()
		}
		return next(c)
	}
}

func startServer() {
	log.Info("Starting at port: " + config.Iris.Port)
	if err := e.Start(":" + config.Iris.Port); err != nil {
		log.Info(err)
	}
}

func waitForInterruptSignal() {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, os.Kill)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("start shutting down")
	if err := e.Shutdown(ctx); err != nil {
		log.Error(err)
	}

	shutdownVips()

	log.Info("finish shutting down")
}

func health(c echo.Context) error {
	log.Debug("HEALTH_CHECK")
	return c.JSON(http.StatusOK, map[string]interface{}{"message": "OK"})
}

func delete(c echo.Context) error {
	url := c.Get(imageStorageURLKey).(string)

	if err := invokeStorageClient(http.MethodDelete, url, nil); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"message": "OK"})
}

func upload(c echo.Context) error {
	log.Debug("Start upload")

	id, err := getImageID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	url := c.Get(imageStorageURLKey).(string)
	objectID := c.Get(objectIDKey).(string)

	body := bytes.NewReader(c.Get(imageDataKey).([]byte))

	if err := invokeStorageClient(http.MethodPut, url, body); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	defer logProcessTime(c, c.Get(startTimeKey).(time.Time))
	// gcs
	return c.JSON(http.StatusOK, map[string]interface{}{
		"image_id":     id,
		"image_width":  c.Get(imageWidthKey),
		"image_height": c.Get(imageHeightKey),
		"image_url":    genGCSURL(config.Storage.GCS.BaseURL, config.Storage.GCS.ImageConfig, objectID),
		"thumb_url":    genGCSURL(config.Storage.GCS.BaseURL, config.Storage.GCS.ThumbConfig, objectID),
	})
}

func invokeStorageClient(method, url string, body io.Reader) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	res, err := storageClient.Do(req)
	if err != nil {
		return err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	return nil
}

func checkDimensions(width, height int) error {
	if config.Image.MinDimension > 0 && (width < config.Image.MinDimension || height < config.Image.MinDimension) {
		return echo.NewHTTPError(http.StatusBadRequest, errSourceDimensionsTooSmall)
	}

	if config.Image.MaxDimension > 0 && (width > config.Image.MaxDimension || height > config.Image.MaxDimension) {
		return echo.NewHTTPError(http.StatusBadRequest, errSourceDimensionsTooBig)
	}

	if width*height > config.Image.MaxResolution {
		return echo.NewHTTPError(http.StatusBadRequest, errSourceDimensionsTooBig)
	}

	return nil
}

func checkTypeAndDimensions(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Debug("Start checkTypeAndDimensions")

		buf := uploadPool.Get(int(c.Get(imageFileSizeKey).(int64)))
		defer func() {
			log.Debug("Put buffer back")
			uploadPool.Put(buf)
		}()

		imgFile := c.Get(imageFileKey).(multipart.File)
		imgconf, imgtypeStr, err := image.DecodeConfig(io.TeeReader(imgFile, buf))
		if err == image.ErrFormat {
			return echo.NewHTTPError(http.StatusBadRequest, errSourceImageTypeNotSupported)
		}
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err)
		}

		imgtype, imgtypeOk := imageTypes[imgtypeStr]
		if !imgtypeOk || !vipsTypeSupportLoad[imgtype] {
			return echo.NewHTTPError(http.StatusBadRequest, errSourceImageTypeNotSupported)
		}

		// this one already returns a http error
		if err = checkDimensions(imgconf.Width, imgconf.Height); err != nil {
			return err
		}

		c.Set(imageWidthKey, imgconf.Width)
		c.Set(imageHeightKey, imgconf.Height)
		c.Set(imageTypeKey, imgtype)

		if _, err := buf.ReadFrom(imgFile); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err)
		}
		log.Debug("Data len: ", buf.Len())
		c.Set(imageDataBufferKey, buf)

		return next(c)
	}
}

func getAndCheckFileSize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Debug("Start getAndCheckFileSize")
		c.Set(startTimeKey, time.Now())

		f, err := c.FormFile("image")
		if err != nil {
			log.Debug(err)
			return echo.NewHTTPError(http.StatusBadRequest, errImageMissing)
		}
		log.Debug("img size: ", f.Size)

		// check the upload file size
		if f.Size > config.Iris.MaxFileSize {
			return echo.NewHTTPError(http.StatusBadRequest, errSourceFileTooBig)
		}

		imgFile, err := f.Open()
		if err != nil {
			log.Debug(err)
			return echo.NewHTTPError(http.StatusBadRequest, errImageMissing)
		}
		defer func() {
			log.Debug("Close upload file")
			imgFile.Close()
		}()

		c.Set(imageFileKey, imgFile)
		c.Set(imageFileSizeKey, f.Size)
		return next(c)
	}
}

func process(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Debug("Start process")

		ctx := context.Background()
		ctx = context.WithValue(ctx, ctxKey(imageTypeKey), c.Get(imageTypeKey))
		ctx = context.WithValue(ctx, ctxKey(imageProcessingOptionsKey), defaultOption)
		ctx = context.WithValue(ctx, ctxKey(imageDataBufferKey), c.Get(imageDataBufferKey))

		newData, processCancel, err := processImage(ctx)
		defer processCancel()
		log.Debug("processed data len: ", len(newData))

		if err != nil {
			if prometheusEnabled {
				incrementPrometheusErrorsTotal("processing")
			}
			return err
		}

		c.Set(imageDataKey, newData)

		return next(c)
	}
}

func genID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Debug("Start genID")

		imageID := fmt.Sprintf("%d.%s", idGen.next(), config.Storage.GCS.Format)
		c.Set(imageIDKey, imageID)

		return next(c)
	}
}

func getImageID(c echo.Context) (string, error) {
	var id string
	if idFromCtx := c.Get(imageIDKey); idFromCtx != nil {
		id = idFromCtx.(string)
	}
	if id == "" {
		id = c.Param("id")
	}
	if id == "" {
		return "", errors.New("NO_IMAGE_ID")
	}
	return id, nil
}

func genObjectURL(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Debug("Start genObjectURL")

		imageID, err := getImageID(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		// Avoid the sequential naming bottleneck
		// https://cloud.google.com/blog/products/gcp/optimizing-your-cloud-storage-performance-google-cloud-performance-atlas
		// That why I add md5(id)-id.[format] to the object name
		imageIDChecksum := fmt.Sprintf("%02x", md5.Sum(([]byte)(imageID[:len(imageID)-4])))
		objectID := fmt.Sprintf("%s-%s", imageIDChecksum, imageID)
		c.Set(objectIDKey, objectID)

		// gcs
		c.Set(imageStorageURLKey, fmt.Sprintf("%s://%s/%s", config.Iris.Storage, config.Storage.GCS.BucketPrefix, objectID))
		return next(c)
	}
}

func getProcessingOptions(ctx context.Context) *processingOptions {
	return ctx.Value(ctxKey(imageProcessingOptionsKey)).(*processingOptions)
}

func getImageType(ctx context.Context) imageType {
	return ctx.Value(ctxKey(imageTypeKey)).(imageType)
}

func getImageDataBuffer(ctx context.Context) *bytes.Buffer {
	return ctx.Value(ctxKey(imageDataBufferKey)).(*bytes.Buffer)
}

// dummy watermark
func watermarkData() ([]byte, imageType, context.CancelFunc, error) {
	return nil, imageTypeUnknown, func() {}, nil
}

func genGCSURL(baseURL, imgConfig string, id string) string {
	source := fmt.Sprintf("%s/%s", config.Storage.GCS.Prefix, id)
	path := fmt.Sprintf("/%s/%s", imgConfig, source)
	if saltKey != nil && secretKey != nil {
		path = signPath(path)
	}
	return fmt.Sprintf("%s/%s", baseURL, path)
}

func signPath(path string) string {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write(saltKey)
	mac.Write([]byte(path))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s%s", signature, path)
}

func logProcessTime(c echo.Context, t time.Time) {
	url := c.Get(imageStorageURLKey).(string)
	width := c.Get(imageWidthKey).(int)
	height := c.Get(imageHeightKey).(int)
	rawSize := c.Get(imageFileSizeKey).(int64)

	processTimeFmt := "Processed option %+v in %d ms: [%s %d %d %d]\n"
	log.Infof(processTimeFmt, *defaultOption, int(time.Since(t).Seconds()*1000), url, width, height, rawSize)
}
