package main

import (
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var (
	// Schema of IRIS configuration
	config = struct {
		PProf struct {
			Enabled bool `mapstructure:"enabled"`
		} `mapstructure:"pprof"`
		Prometheus struct {
			Enabled bool `mapstructure:"enabled"`
		} `mapstructure:"prometheus"`
		Iris struct {
			Port                           string        `mapstructure:"port"`
			MaxConn                        int           `mapstructure:"max_conns"`
			Storage                        string        `mapstructure:"storage"`
			Concurrency                    int           `mapstructure:"concurrency"`
			Timeout                        time.Duration `mapstructure:"timeout"`
			FreeMemoryInterval             time.Duration `mapstructure:"free_mem_interval"`
			LogMemStats                    bool          `mapstructure:"log_mem_stats"`
			IgnoreSslVerification          bool          `mapstructure:"ignore_ssl_verification"`
			MaxFileSize                    int64         `mapstructure:"max_file_size"`
			BufferSize                     int           `mapstructure:"upload_buffer_size"`
			BufferPoolCalibrationThreshold int           `mapstructure:"buffer_pool_calib_threshold"`
		} `mapstructure:"one_image"`
		Image struct {
			Width            int     `mapstructure:"width"`
			Height           int     `mapstructure:"height"`
			Quality          int     `mapstructure:"quality"`
			Type             string  `mapstructure:"type"`
			MaxDimension     int     `mapstructure:"max_dimension"`
			MinDimension     int     `mapstructure:"min_dimension"`
			MaxResolution    int     `mapstructure:"max_resolution"`
			JpegProgressive  bool    `mapstructure:"jpeg_progressive"`
			PngInterlaced    bool    `mapstructure:"png_interlaced"`
			WatermarkOpacity float64 `mapstructure:"watermark_opacity"`
			MaxGifFrames     int     `mapstructure:"max_gif_frames"`
		} `mapstructure:"image"`
		Storage struct {
			GCS struct {
				Enabled      bool   `mapstructure:"enabled"`
				BucketPrefix string `mapstructure:"bucket_prefix"`
				BaseURL      string `mapstructure:"base_url"`
				Prefix       string `mapstructure:"prefix"`
				ImageConfig  string `mapstructure:"image_config"`
				ThumbConfig  string `mapstructure:"thumb_config"`
				Format       string `mapstructure:"format"`
			} `mapstructure: "gcs"`
		} `mapstructure:"storage"`
	}{}
)

// initialize configuration
func init() {
	// Initialize viper default instance with API base config.
	v := viper.New()
	v.SetConfigName("config")        // Name of config file (without extension).
	v.AddConfigPath(".")             // Look for config in current directory
	v.AddConfigPath("/app")          // Look for config in current directory
	v.AddConfigPath("./config")      // Optionally look for config in the working directory.
	v.AddConfigPath("../config/")    // Look for config needed for tests.
	v.AddConfigPath("../../config/") // Look for config needed for tests.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "__"))
	v.AutomaticEnv()
	// Initialize map that contains viper configuration objects.
	// Find and read the config file
	if err := v.ReadInConfig(); err != nil {
		// Handle errors reading the config file
		log.Fatalf("error config file: %s", err.Error())
	}

	if err := v.Unmarshal(&config); err != nil {
		// Handle errors reading the config file
		log.Fatalf("fatal error config file: %s", err)
	}

	if config.Iris.Concurrency == 0 {
		config.Iris.Concurrency = runtime.NumCPU() * 2
	}

	if config.Iris.MaxConn == 0 {
		config.Iris.MaxConn = config.Iris.Concurrency * 10
	}

	log.Infof("Current configuration: %+v", config)
}
