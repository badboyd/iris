FROM alpine:edge

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$PATH

# Install dependencies
RUN echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
  && apk --no-cache upgrade \
  && apk add --no-cache curl ca-certificates go gcc g++ make musl-dev fftw-dev orc-dev glib-dev expat-dev \
    libjpeg-turbo-dev libpng-dev libwebp-dev giflib-dev librsvg-dev libexif-dev lcms2-dev

# Build ImageMagick
RUN cd /root \
  && mkdir ImageMagick \
  && curl -Ls https://imagemagick.org/download/ImageMagick.tar.gz | tar -xz -C ImageMagick --strip-components 1 \
  && cd ImageMagick \
  && ./configure \
    --enable-silent-rules \
    --disable-static \
    --disable-openmp \
    --disable-deprecated \
    --disable-docs \
    --with-threads \
    --without-magick-plus-plus \
    --without-utilities \
    --without-perl \
    --without-bzlib \
    --without-dps \
    --without-freetype \
    --without-jbig \
    --without-jpeg \
    --without-lcms \
    --without-lzma \
    --without-png \
    --without-tiff \
    --without-wmf \
    --without-xml \
    --without-webp \
  && make install-strip

# Build libvips
RUN cd /root \
  && export VIPS_VERSION=$(curl -s "https://api.github.com/repos/libvips/libvips/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/') \
  && echo "Vips version: $VIPS_VERSION" \
  && curl -Ls https://github.com/libvips/libvips/releases/download/v$VIPS_VERSION/vips-$VIPS_VERSION.tar.gz | tar -xz \
  && cd vips-$VIPS_VERSION \
  && ./configure \
    --disable-magickload \
    --without-python \
    --without-tiff \
    --without-OpenEXR \
    --enable-debug=no \
    --disable-static \
    --enable-silent-rules \
  && make install-strip


ADD . /go/src/github.com/701search/iris
WORKDIR /go/src/github.com/701search/iris

# Build imgproxy
RUN cd /go/src/github.com/701search/iris \
  && CGO_LDFLAGS_ALLOW="-s|-w|-l" go build -v -o /usr/local/bin/iris

# Copy compiled libs here to copy them to the final image
RUN cd /root \
  && mkdir libs \
  && ldd /usr/local/bin/iris | grep /usr/local/lib/ | awk '{print $3}' | xargs -I '{}' cp '{}' libs/

# ==================================================================================================
# Final image

FROM alpine:edge

RUN echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
  && apk --no-cache upgrade \
  && apk add --no-cache bash ca-certificates fftw orc glib expat libjpeg-turbo libpng \
    libwebp giflib librsvg libgsf libexif lcms2 \
  && rm -rf /var/cache/apk*

COPY --from=0 /usr/local/bin/iris /usr/local/bin/
COPY --from=0 /root/libs/* /usr/local/lib/

COPY --from=0 /go/src/github.com/701search/iris/config.yaml /
COPY --from=0 /go/src/github.com/701search/iris/config.yaml /app/config

CMD ["iris"]

EXPOSE 8080
