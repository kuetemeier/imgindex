package imgmeta

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// BasicInfo contains the most basic information that could be asked for
type BasicInfo struct {
	Width    interface{}
	Height   uint32
	Title    string
	Descr    string
	Keywords []string
}

// GetBasicInfo gets the basic information from the meta-information of the image
func GetBasicInfo(img Image) (info BasicInfo) {
	width, err := img.ReadTagValue("SOF0", SOF0ImageWidth)
	if err == nil {
		info.Width = width
	} else {
		log.Error(err.Error())
	}
	height, err := img.ReadTagValue("SOF0", SOF0ImageHeight)
	if err == nil {
		info.Height = height.(uint32)
	} else {
		log.Error(err.Error())
	}
	keyword, err := img.ReadTagValue("IPTC", IptcTagApplication2Keywords)
	if err == nil {
		info.Keywords = []string{keyword.(string)}
	}
	datetime, err := img.ReadTagValue("EXIF", ExifTagDateTimeOriginal)
	if err == nil {
		log.Error(fmt.Sprintf("datetime:%v\n", datetime))
	}
	//height, err := img.ReadTagValue("IPTC", IptcTagApplication2Keywords)
	//if err == nil {
	//	info.Height = height.(float64)
	//} else {
	//	log.Error(err.Error())
	//}
	return
}
