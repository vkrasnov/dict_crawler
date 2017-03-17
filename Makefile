all:
	go get github.com/fedesog/webdriver
	go get github.com/vkrasnov/dictator
	go get github.com/tealeg/xlsx
	curl https://chromedriver.storage.googleapis.com/2.28/chromedriver_mac64.zip > cd_mac.zip
	unzip cd_mac.zip
	rm cd_mac.zip
	go build

linux:
	go get github.com/fedesog/webdriver
	go get github.com/vkrasnov/dictator
	go get github.com/tealeg/xlsx
	curl https://chromedriver.storage.googleapis.com/2.28/chromedriver_linux64.zip > cd_lin.zip
	unzip cd_lin.zip
	rm cd_lin.zip
	go build
