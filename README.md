# Setup

## Running virtual RTSP camera

Run docker image:

```bash
docker run -p 554:8554 -d -v $(pwd)/virtualRTSP/samples:/samples -d -e SOURCE_URL=file:///samples/video1.mp4 kerberos/virtual-rtsp:1.0.6 
```

## Compile openCV for go

```bash
go get -u -d gocv.io/x/gocv
cd $GOPATH/src/gocv.io/x/gocv
make install
```

## Compile tesseract for go

```bash
wget https://github.com/tesseract-ocr/tesseract/archive/3.05.02.tar.gz
tar -xvzf ./3.05.02.tar.gz 
sudo apt-get install libleptonica-dev
cd tesseract-3.05.02/
./autogen.sh
./configure --enable-debug LDFLAGS="-L/usr/local/lib" CFLAGS="-I/usr/local/include"
make
sudo make install
sudo make install-langs
sudo ldconfig
```

## Compile openALPR for go

```bash
sudo add-apt-repository ppa:xapienz/curl34
sudo apt-get update
sudo apt-get install libcurl4 libcurl4-openssl-dev liblog4cplus-dev libtesseract-dev 
git clone https://github.com/openalpr/openalpr.git
cd openalpr/src
mkdir build
cd build
cmake -DCMAKE_INSTALL_PREFIX:PATH=/usr -DCMAKE_INSTALL_SYSCONFDIR:PATH=/etc ..
make
sudo make install
```
