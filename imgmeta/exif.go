package imgmeta

import (
	"encoding/binary"
	"fmt"
	"math"

	log "github.com/sirupsen/logrus"
)

/*
Exif APP1 segments are made up by an identifier, a TIFF header and a sequence of IFDs (Image File Directories) and subIFDs.
The high level IFDs are only two (IFD0, for photographic parameters, and IFD1 for thumbnail parameters); they can be
followed by thumbnail data. The structure is as follows:

    [Record name]    [size]   [description]
    ---------------------------------------
    Identifier       6 bytes   ("Exif\000\000" = 0x457869660000), not stored
    Endianness       2 bytes   'II' (little-endian) or 'MM' (big-endian)
    Signature        2 bytes   a fixed value = 42
    IFD0_Pointer     4 bytes   offset of 0th IFD (usually 8), not stored
    IFD0                ...    main image IFD
    IFD0@SubIFD         ...    Exif private tags (optional, linked by IFD0)
    IFD0@SubIFD@Interop ...    Interoperability IFD (optional,linked by SubIFD)
    IFD0@GPS            ...    GPS IFD (optional, linked by IFD0)
    APP1@IFD1           ...    thumbnail IFD (optional, pointed to by IFD0)
    ThumbnailData       ...    Thumbnail image (optional, 0xffd8.....ffd9)

So, each Exif APP1 segment starts with the identifier string "Exif\000\000"; this avoids a conflict with other applications
using APP1, for instance XMP data. The three following fields (Endianness, Signature and IFD0_Pointer) constitute the so
called TIFF header.
The offset of the 0th IFD in the TIFF header, as well as IFD links in the following IFDs, is given with respect to the
beginning of the TIFF header (i.e. the address of the 'MM' or 'II' pair). This means that if the 0th IFD begins (as usual)
immediately after the end of the TIFF header, the offset value is 8. An Exif segment is the only part of a JPEG file whose
endianness is not fixed to big-endian.

If the thumbnail is present it is located after the 1st IFD. There are 3 possible formats: JPEG (only this is compressed),
RGB TIFF, and YCbCr TIFF. It seems that JPEG and 160x120 pixels are recommended for Exif ver. 2.1 or higher (mandatory for DCF files).
Since the segment size for a segment is recorded in 2 bytes, thumbnails are limited to slightly less than 64KB.

Each IFD block is a structured sequence of records, called, in the Exif jargon, Interoperability arrays.
The beginning of the 0th IFD is given by the 'IFD0_Pointer' value. The structure of an IFD is the following:

    [Record name]    [size]   [description]
    ---------------------------------------
                     2 bytes  number n of Interoperability arrays
                   12n bytes  the n arrays (12 bytes each)
                     4 bytes  link to next IFD (can be zero)
                       ...    additional data area

The next_link field of the 0th IFD, if non-null, points to the beginning of the 1st IFD. The 1st IFD as well as all other sub-IFDs
must have next_link set to zero. The thumbnail location and size is given by some interoperability arrays in the 1st IFD.
The structure of an Interoperability array is:

    [Record name]    [size]   [description]
    ---------------------------------------
                     2 bytes  Tag (a unique 2-byte number)
                     2 bytes  Type (one out of 12 types)
                     4 bytes  Count (the number of values)
                     4 bytes  Value Offset (value or offset)

The possible types are the same as for the Record class, exception made for nibbles and references. Indeed, the Record class is
modelled after interoperability arrays, and each interoperability array gets stored as a Record with given tag, type, count and
values. The "value offset" field gives the offset from the TIFF header base where the value is recorded.
It contains the actual value if it is not larger than 4 bytes (32 bits). If the value is shorter than 4 bytes, it is recorded
in the lower end of the 4-byte area (smaller offsets).

*/

// ============================================== EXIF =======================================================

type tEXIFAPP struct {
	offset uint64           // Offset of this APP in the file
	endian binary.ByteOrder // TIFF-Header, Byte-Order
	block  []byte           // full APP block
}

func (t tEXIFAPP) Name() string {
	return "EXIF"
}
func (t tEXIFAPP) Marker() uint16 {
	return t.endian.Uint16(t.block)
}
func (t tEXIFAPP) Length() uint16 {
	return t.endian.Uint16(t.block[2:])
}
func (t tEXIFAPP) ID(cid []byte) (id []byte) {
	id = t.block[4 : 4+len(cid)]
	return
}
func (t tEXIFAPP) HasID(cid []byte) bool {
	id := t.block[4 : 4+len(cid)]
	for i, b := range id {
		if b != cid[i] {
			return false
		}
	}
	return true
}

func (t tEXIFAPP) TIFFByteOrder() binary.ByteOrder {
	bo := binary.BigEndian.Uint16(t.block[10:12])
	if bo == cINTEL {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

func (t tEXIFAPP) TIFFOffsetToIFD0() uint32 {
	endian := t.TIFFByteOrder()
	return endian.Uint32(t.block[14:18])
}

type ifdOffsetItem struct {
	offset  uint32
	ifdType uint16
}

func (t tEXIFAPP) ReadValue(tagID2Find uint16) (interface{}, error) {
	log.Debug(fmt.Sprintf("Read value of tag:0x%X in APP:EXIF\n", tagID2Find))

	tiffOffset := uint32(10)
	ifd0Offset := tiffOffset + t.TIFFOffsetToIFD0()
	endian := t.TIFFByteOrder()

	ifdQueue := []ifdOffsetItem{}
	ifdQueue = append(ifdQueue, ifdOffsetItem{offset: ifd0Offset, ifdType: cIFDZERO})

	for len(ifdQueue) > 0 {
		// Pop the next offset to process
		ifdItem := ifdQueue[len(ifdQueue)-1]
		ifdQueue = ifdQueue[:len(ifdQueue)-1]

		ifd := tExifIFD{offset: ifdItem.offset, appblock: t.block, endian: endian}
		// How many fields does this IFD have ?
		numberOfTags := ifd.NumberOfTags()

		for i := uint32(0); i < numberOfTags; i++ {
			tag := ifd.GetTag(i)
			tagID := tag.TagID()

			if tagID == tagID2Find {
				return ifd.ReadValue(tag)
			}

			// IFD0, reading the offsets to the other IFD segments
			if ifdItem.ifdType == cIFDZERO && tagID == cIFDEXIF {
				anotherIfdOffset := tiffOffset + tag.valueOrOffset()
				ifdQueue = append(ifdQueue, ifdOffsetItem{offset: anotherIfdOffset, ifdType: cIFDEXIF})
			} else if ifdItem.ifdType == cIFDZERO && tagID == cIFDGPS {
				anotherIfdOffset := tiffOffset + tag.valueOrOffset()
				ifdQueue = append(ifdQueue, ifdOffsetItem{offset: anotherIfdOffset, ifdType: cIFDGPS})
			} else if ifdItem.ifdType == cIFDEXIF && tagID == cIFDINTEROP {
				anotherIfdOffset := tiffOffset + tag.valueOrOffset()
				ifdQueue = append(ifdQueue, ifdOffsetItem{offset: anotherIfdOffset, ifdType: cIFDINTEROP})
			}
		}
	}

	return int(1), nil
}

type tExifIFD struct {
	offset   uint32           // IFD-Offset
	endian   binary.ByteOrder // Endian
	appblock []byte
}

func (ifd tExifIFD) NumberOfTags() uint32 {
	return uint32(ifd.endian.Uint16(ifd.appblock[ifd.offset:]))
}

func (ifd tExifIFD) GetTag(index uint32) tExifTag {
	o := ifd.offset + 2 + (index * 12)
	return tExifTag{appblock: ifd.appblock[o : o+12], endian: ifd.endian}
}

func (ifd tExifIFD) FindTag(id uint16) (tExifTag, bool) {
	n := ifd.NumberOfTags()
	for i := uint32(0); i < n; i++ {
		tag := ifd.GetTag(i)
		if tag.TagID() == id {
			return tag, true
		}
	}
	return tExifTag{appblock: ifd.appblock[0:0], endian: ifd.endian}, false
}

type tExifTag struct {
	endian   binary.ByteOrder
	offset   uint32
	appblock []byte
}

func (tag tExifTag) TagID() uint16 {
	return tag.endian.Uint16(tag.appblock[tag.offset:])
}
func (tag tExifTag) TypeID() uint16 {
	if tag.countOrComponents() > 1 {
		return uint16(cARRAY) | tag.endian.Uint16(tag.appblock[tag.offset+2:])
	}
	return tag.endian.Uint16(tag.appblock[tag.offset+2:])
}
func (tag tExifTag) countOrComponents() uint32 {
	return tag.endian.Uint32(tag.appblock[tag.offset+4:])
}
func (tag tExifTag) valueOrOffset() uint32 {
	return tag.endian.Uint32(tag.appblock[tag.offset+8:])
}
func (tag tExifTag) valueAsU8() uint8 {
	return tag.appblock[tag.offset+8+3]
}
func (tag tExifTag) valueAsU16() uint16 {
	return binary.LittleEndian.Uint16(tag.appblock[tag.offset+8+2:])
}
func (tag tExifTag) valueAsU32() uint32 {
	return binary.LittleEndian.Uint32(tag.appblock[tag.offset+8:])
}
func (tag tExifTag) valueAsFloat32() float32 {
	bits := binary.LittleEndian.Uint32(tag.appblock[tag.offset+8:])
	float := math.Float32frombits(bits)
	return float
}

type tExifTagFieldType uint16

var aExifTagFieldSize = []int{0, 1, 1, 2, 4, 8, 1, 1, 2, 4, 8, 4, 8}

func getExifTagFieldSize(fieldType tExifTagFieldType) int {
	return aExifTagFieldSize[int(fieldType)]
}

const (
	cARRAY     = 0x0100
	cUBYTE     = 0x0001
	cASCII     = 0x0002
	cUSHORT    = 0x0003
	cULONG     = 0x0004
	cURATIONAL = 0x0005
	cSBYTE     = 0x0006
	cUNDEFINED = 0x0007
	cSSHORT    = 0x0008
	cSLONG     = 0x0009
	cSRATIONAL = 0x000A
	cFLOAT32   = 0x000B
	cFLOAT64   = 0x000C
)

func (ifd tExifIFD) readValueFromOffset(offset uint32, typeID uint16, count uint32) (interface{}, error) {
	switch typeID {
	case cARRAY | cUBYTE:
		offset = ifd.offset + offset
		array := append([]uint8{}, ifd.appblock[offset:offset+count]...)
		return array, nil
	case cARRAY | cUSHORT:
		offset = ifd.offset + offset
		block := ifd.appblock[offset : offset+count*2]
		array := make([]uint16, count, count)
		for i := uint32(0); i < count; i++ {
			array[i] = ifd.endian.Uint16(block[i*2:])
		}
		return array, nil
	case cARRAY | cULONG:
		offset = ifd.offset + offset
		block := ifd.appblock[offset : offset+count*4]
		array := make([]uint32, count, count)
		for i := uint32(0); i < count; i++ {
			array[i] = ifd.endian.Uint32(block[i*4:])
		}
		return array, nil
	case cARRAY | cSBYTE:
		offset = ifd.offset + offset
		block := ifd.appblock[offset : offset+count]
		array := make([]int8, count, count)
		for i := uint32(0); i < count; i++ {
			array[i] = int8(block[i])
		}
		return array, nil
	case cARRAY | cSSHORT:
		offset = ifd.offset + offset
		block := ifd.appblock[offset : offset+count*2]
		array := make([]int16, count, count)
		for i := uint32(0); i < count; i++ {
			array[i] = int16(ifd.endian.Uint16(block[i*2:]))
		}
		return array, nil
	case cARRAY | cSLONG:
		offset = ifd.offset + offset
		block := ifd.appblock[offset : offset+count*4]
		array := make([]int32, count, count)
		for i := uint32(0); i < count; i++ {
			array[i] = int32(ifd.endian.Uint32(block[i*4:]))
		}
		return array, nil
	case cFLOAT64:
		bits := ifd.endian.Uint64(ifd.appblock[ifd.offset+offset:])
		float := math.Float64frombits(bits)
		return float, nil
	case cURATIONAL:
		numerator := ifd.endian.Uint32(ifd.appblock[ifd.offset+offset:])
		denominator := ifd.endian.Uint32(ifd.appblock[ifd.offset+offset+4:])
		return float64(numerator) / float64(denominator), nil
	case cSRATIONAL:
		numerator := int32(ifd.endian.Uint32(ifd.appblock[ifd.offset+offset:]))
		denominator := int32(ifd.endian.Uint32(ifd.appblock[ifd.offset+offset+4:]))
		return float64(numerator) / float64(denominator), nil
	}
	return int(0), &exifError{"Reading EXIF tag value from offset failed"}
}

func (ifd tExifIFD) ReadValue(tag tExifTag) (interface{}, error) {

	switch tag.TypeID() {
	case cUBYTE:
		return uint8(tag.valueOrOffset()), nil
	case cUSHORT:
		return uint16(tag.valueOrOffset()), nil
	case cULONG:
		return uint32(tag.valueOrOffset()), nil
	case cSBYTE:
		return int8(tag.valueOrOffset()), nil
	case cSSHORT:
		return int16(tag.valueOrOffset()), nil
	case cSLONG:
		return int32(tag.valueOrOffset()), nil
	case cFLOAT32:
		return tag.valueAsFloat32(), nil
	case cFLOAT64:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cURATIONAL:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cSRATIONAL:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cARRAY | cUBYTE:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cARRAY | cUSHORT:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cARRAY | cULONG:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cARRAY | cSBYTE:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cARRAY | cSSHORT:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	case cARRAY | cSLONG:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID(), tag.countOrComponents())
	}
	return int(0), &exifError{"Reading EXIF tag value failed"}
}

const (
	ExifTagImageWidth                  uint16 = 0x100
	ExifTagImageHeight                 uint16 = 0x101
	ExifTagBitsPerSample               uint16 = 0x102
	ExifTagCompression                 uint16 = 0x103
	ExifTagPhotometricInterpretation   uint16 = 0x106
	ExifTagImageDescription            uint16 = 0x10E
	ExifTagMake                        uint16 = 0x10F
	ExifTagModel                       uint16 = 0x110
	ExifTagStripOffsets                uint16 = 0x111
	ExifTagOrientation                 uint16 = 0x112
	ExifTagSamplesPerPixel             uint16 = 0x115
	ExifTagRowsPerStrip                uint16 = 0x116
	ExifTagStripByteCounts             uint16 = 0x117
	ExifTagXResolution                 uint16 = 0x11A
	ExifTagYResolution                 uint16 = 0x11B
	ExifTagPlanarConfiguration         uint16 = 0x11C
	ExifTagResolutionUnit              uint16 = 0x128
	ExifTagTransferFunction            uint16 = 0x12D
	ExifTagSoftware                    uint16 = 0x131
	ExifTagDateTime                    uint16 = 0x132
	ExifTagArtist                      uint16 = 0x13B
	ExifTagWhitePoint                  uint16 = 0x13E
	ExifTagPrimaryChromaticities       uint16 = 0x13F
	ExifTagJPEGInterchangeFormat       uint16 = 0x201
	ExifTagJPEGInterchangeFormatLength uint16 = 0x202
	ExifTagYCbCrCoefficients           uint16 = 0x211
	ExifTagYCbCrSubSampling            uint16 = 0x212
	ExifTagYCbCrPositioning            uint16 = 0x213
	ExifTagReferenceBlackWhite         uint16 = 0x214
	ExifTagCopyright                   uint16 = 0x8298

	ExifTagExposureTime              uint16 = 0x829A
	ExifTagFNumber                   uint16 = 0x829D
	ExifTagExposureProgram           uint16 = 0x8822
	ExifTagSpectralSensitivity       uint16 = 0x8824
	ExifTagPhotographicSensitivity   uint16 = 0x8827
	ExifTagOECF                      uint16 = 0x8828
	ExifTagSensitivityType           uint16 = 0x8830
	ExifTagStandardOutputSensitivity uint16 = 0x8831
	ExifTagRecommendedExposureIndex  uint16 = 0x8832
	ExifTagISOSpeed                  uint16 = 0x8833
	ExifTagISOSpeedLatitudeyyy       uint16 = 0x8834
	ExifTagISOSpeedLatitudezzz       uint16 = 0x8835
	ExifTagExifVersion               uint16 = 0x9000
	ExifTagDateTimeOriginal          uint16 = 0x9003
	ExifTagDateTimeDigitized         uint16 = 0x9004
	ExifTagComponentsConfiguration   uint16 = 0x9101
	ExifTagCompressedBitsPerPixel    uint16 = 0x9102
	ExifTagShutterSpeedValue         uint16 = 0x9201
	ExifTagApertureValue             uint16 = 0x9202
	ExifTagBrightnessValue           uint16 = 0x9203
	ExifTagExposureBiasValue         uint16 = 0x9204
	ExifTagMaxApertureValue          uint16 = 0x9205
	ExifTagSubjectDistance           uint16 = 0x9206
	ExifTagMeteringMode              uint16 = 0x9207
	ExifTagLightSource               uint16 = 0x9208
	ExifTagFlash                     uint16 = 0x9209
	ExifTagFocalLength               uint16 = 0x920A
	ExifTagSubjectArea               uint16 = 0x9214
	ExifTagMakerNote                 uint16 = 0x927C
	ExifTagUserComment               uint16 = 0x9286
	ExifTagSubsecTime                uint16 = 0x9290
	ExifTagSubsecTimeOriginal        uint16 = 0x9291
	ExifTagSubsecTimeDigitized       uint16 = 0x9292
	ExifTagFlashpixVersion           uint16 = 0xA000
	ExifTagColorSpace                uint16 = 0xA001
	ExifTagPixelXDimension           uint16 = 0xA002
	ExifTagPixelYDimension           uint16 = 0xA003
	ExifTagRelatedSoundFile          uint16 = 0xA004
	ExifTagFlashEnergy               uint16 = 0xA20B
	ExifTagSpatialFrequencyResponse  uint16 = 0xA20C
	ExifTagFocalPlaneXResolution     uint16 = 0xA20E
	ExifTagFocalPlaneYResolution     uint16 = 0xA20F
	ExifTagFocalPlaneResolutionUnit  uint16 = 0xA210
	ExifTagSubjectLocation           uint16 = 0xA214
	ExifTagExposureIndex             uint16 = 0xA215
	ExifTagSensingMethod             uint16 = 0xA217
	ExifTagFileSource                uint16 = 0xA300
	ExifTagSceneType                 uint16 = 0xA301
	ExifTagCFAPattern                uint16 = 0xA302
	ExifTagCustomRendered            uint16 = 0xA401
	ExifTagExposureMode              uint16 = 0xA402
	ExifTagWhiteBalance              uint16 = 0xA403
	ExifTagDigitalZoomRatio          uint16 = 0xA404
	ExifTagFocalLengthIn35mmFilm     uint16 = 0xA405
	ExifTagSceneCaptureType          uint16 = 0xA406
	ExifTagGainControl               uint16 = 0xA407
	ExifTagContrast                  uint16 = 0xA408
	ExifTagSaturation                uint16 = 0xA409
	ExifTagSharpness                 uint16 = 0xA40A
	ExifTagDeviceSettingDescription  uint16 = 0xA40B
	ExifTagSubjectDistanceRange      uint16 = 0xA40C
	ExifTagImageUniqueID             uint16 = 0xA420
	ExifTagCameraOwnerName           uint16 = 0xA430
	ExifTagBodySerialNumber          uint16 = 0xA431
	ExifTagLensSpecification         uint16 = 0xA432
	ExifTagLensMake                  uint16 = 0xA433
	ExifTagLensModel                 uint16 = 0xA434
	ExifTagLensSerialNumber          uint16 = 0xA435

	ExifGpsTagGPSVersionID         uint16 = 0x0
	ExifGpsTagGPSLatitudeRef       uint16 = 0x1
	ExifGpsTagGPSLatitude          uint16 = 0x2
	ExifGpsTagGPSLongitudeRef      uint16 = 0x3
	ExifGpsTagGPSLongitude         uint16 = 0x4
	ExifGpsTagGPSAltitudeRef       uint16 = 0x5
	ExifGpsTagGPSAltitude          uint16 = 0x6
	ExifGpsTagGPSTimestamp         uint16 = 0x7
	ExifGpsTagGPSSatellites        uint16 = 0x8
	ExifGpsTagGPSStatus            uint16 = 0x9
	ExifGpsTagGPSMeasureMode       uint16 = 0xA
	ExifGpsTagGPSDOP               uint16 = 0xB
	ExifGpsTagGPSSpeedRef          uint16 = 0xC
	ExifGpsTagGPSSpeed             uint16 = 0xD
	ExifGpsTagGPSTrackRef          uint16 = 0xE
	ExifGpsTagGPSTrack             uint16 = 0xF
	ExifGpsTagGPSImgDirectionRef   uint16 = 0x10
	ExifGpsTagGPSImgDirection      uint16 = 0x11
	ExifGpsTagGPSMapDatum          uint16 = 0x12
	ExifGpsTagGPSDestLatitudeRef   uint16 = 0x13
	ExifGpsTagGPSDestLatitude      uint16 = 0x14
	ExifGpsTagGPSDestLongitudeRef  uint16 = 0x15
	ExifGpsTagGPSDestLongitude     uint16 = 0x16
	ExifGpsTagGPSDestBearingRef    uint16 = 0x17
	ExifGpsTagGPSDestBearing       uint16 = 0x18
	ExifGpsTagGPSDestDistanceRef   uint16 = 0x19
	ExifGpsTagGPSDestDistance      uint16 = 0x1A
	ExifGpsTagGPSProcessingMethod  uint16 = 0x1B
	ExifGpsTagGPSAreaInformation   uint16 = 0x1C
	ExifGpsTagGPSDateStamp         uint16 = 0x1D
	ExifGpsTagGPSDifferential      uint16 = 0x1E
	ExifGpsTagGPSHPositioningError uint16 = 0x1F

	ExifXpTagXPTitle    uint16 = 0x9c9b
	ExifXpTagXPComment  uint16 = 0x9c9c
	ExifXpTagXPAuthor   uint16 = 0x9c9d
	ExifXpTagXPKeywords uint16 = 0x9c9e
	ExifXpTagXPSubject  uint16 = 0x9c9f
)

type tExifTagDescr struct {
	tag  uint16
	id   uint16
	name string
}

var aExifTagDescr = map[uint16]tExifTagDescr{
	// Primary tags
	ExifTagImageWidth:                  {tag: cIFDZERO, name: "ImageWidth", id: ExifTagImageWidth},
	ExifTagImageHeight:                 {tag: cIFDZERO, name: "ImageLength", id: ExifTagImageHeight},
	ExifTagBitsPerSample:               {tag: cIFDZERO, name: "BitsPerSample", id: ExifTagBitsPerSample},
	ExifTagCompression:                 {tag: cIFDZERO, name: "Compression", id: ExifTagCompression},
	ExifTagPhotometricInterpretation:   {tag: cIFDZERO, name: "PhotometricInterpretation", id: ExifTagPhotometricInterpretation},
	ExifTagImageDescription:            {tag: cIFDZERO, name: "ImageDescription", id: ExifTagImageDescription},
	ExifTagMake:                        {tag: cIFDZERO, name: "Make", id: ExifTagMake},
	ExifTagModel:                       {tag: cIFDZERO, name: "Model", id: ExifTagModel},
	ExifTagStripOffsets:                {tag: cIFDZERO, name: "StripOffsets", id: ExifTagStripOffsets},
	ExifTagOrientation:                 {tag: cIFDZERO, name: "Orientation", id: ExifTagOrientation},
	ExifTagSamplesPerPixel:             {tag: cIFDZERO, name: "SamplesPerPixel", id: ExifTagSamplesPerPixel},
	ExifTagRowsPerStrip:                {tag: cIFDZERO, name: "RowsPerStrip", id: ExifTagRowsPerStrip},
	ExifTagStripByteCounts:             {tag: cIFDZERO, name: "StripByteCounts", id: ExifTagStripByteCounts},
	ExifTagXResolution:                 {tag: cIFDZERO, name: "XResolution", id: ExifTagXResolution},
	ExifTagYResolution:                 {tag: cIFDZERO, name: "YResolution", id: ExifTagYResolution},
	ExifTagPlanarConfiguration:         {tag: cIFDZERO, name: "PlanarConfiguration", id: ExifTagPlanarConfiguration},
	ExifTagResolutionUnit:              {tag: cIFDZERO, name: "ResolutionUnit", id: ExifTagResolutionUnit},
	ExifTagTransferFunction:            {tag: cIFDZERO, name: "TransferFunction", id: ExifTagTransferFunction},
	ExifTagSoftware:                    {tag: cIFDZERO, name: "Software", id: ExifTagSoftware},
	ExifTagDateTime:                    {tag: cIFDZERO, name: "DateTime", id: ExifTagDateTime},
	ExifTagArtist:                      {tag: cIFDZERO, name: "Artist", id: ExifTagArtist},
	ExifTagWhitePoint:                  {tag: cIFDZERO, name: "WhitePoint", id: ExifTagWhitePoint},
	ExifTagPrimaryChromaticities:       {tag: cIFDZERO, name: "PrimaryChromaticities", id: ExifTagPrimaryChromaticities},
	ExifTagJPEGInterchangeFormat:       {tag: cIFDZERO, name: "JPEGInterchangeFormat", id: ExifTagJPEGInterchangeFormat},
	ExifTagJPEGInterchangeFormatLength: {tag: cIFDZERO, name: "JPEGInterchangeFormatLength", id: ExifTagJPEGInterchangeFormatLength},
	ExifTagYCbCrCoefficients:           {tag: cIFDZERO, name: "YCbCrCoefficients", id: ExifTagYCbCrCoefficients},
	ExifTagYCbCrSubSampling:            {tag: cIFDZERO, name: "YCbCrSubSampling", id: ExifTagYCbCrSubSampling},
	ExifTagYCbCrPositioning:            {tag: cIFDZERO, name: "YCbCrPositioning", id: ExifTagYCbCrPositioning},
	ExifTagReferenceBlackWhite:         {tag: cIFDZERO, name: "ReferenceBlackWhite", id: ExifTagReferenceBlackWhite},
	ExifTagCopyright:                   {tag: cIFDZERO, name: "Copyright", id: ExifTagCopyright},

	// EXIF tags
	ExifTagExposureTime:              {tag: cIFDEXIF, name: "ExposureTime", id: ExifTagExposureTime},
	ExifTagFNumber:                   {tag: cIFDEXIF, name: "FNumber", id: ExifTagFNumber},
	ExifTagExposureProgram:           {tag: cIFDEXIF, name: "ExposureProgram", id: ExifTagExposureProgram},
	ExifTagSpectralSensitivity:       {tag: cIFDEXIF, name: "SpectralSensitivity", id: ExifTagSpectralSensitivity},
	ExifTagPhotographicSensitivity:   {tag: cIFDEXIF, name: "PhotographicSensitivity", id: ExifTagPhotographicSensitivity},
	ExifTagOECF:                      {tag: cIFDEXIF, name: "OECF", id: ExifTagOECF},
	ExifTagSensitivityType:           {tag: cIFDEXIF, name: "SensitivityType", id: ExifTagSensitivityType},
	ExifTagStandardOutputSensitivity: {tag: cIFDEXIF, name: "StandardOutputSensitivity", id: ExifTagStandardOutputSensitivity},
	ExifTagRecommendedExposureIndex:  {tag: cIFDEXIF, name: "RecommendedExposureIndex", id: ExifTagRecommendedExposureIndex},
	ExifTagISOSpeed:                  {tag: cIFDEXIF, name: "ISOSpeed", id: ExifTagISOSpeed},
	ExifTagISOSpeedLatitudeyyy:       {tag: cIFDEXIF, name: "ISOSpeedLatitudeyyy", id: ExifTagISOSpeedLatitudeyyy},
	ExifTagISOSpeedLatitudezzz:       {tag: cIFDEXIF, name: "ISOSpeedLatitudezzz", id: ExifTagISOSpeedLatitudezzz},
	ExifTagExifVersion:               {tag: cIFDEXIF, name: "ExifVersion", id: ExifTagExifVersion},
	ExifTagDateTimeOriginal:          {tag: cIFDEXIF, name: "DateTimeOriginal", id: ExifTagDateTimeOriginal},
	ExifTagDateTimeDigitized:         {tag: cIFDEXIF, name: "DateTimeDigitized", id: ExifTagDateTimeDigitized},
	ExifTagComponentsConfiguration:   {tag: cIFDEXIF, name: "ComponentsConfiguration", id: ExifTagComponentsConfiguration},
	ExifTagCompressedBitsPerPixel:    {tag: cIFDEXIF, name: "CompressedBitsPerPixel", id: ExifTagCompressedBitsPerPixel},
	ExifTagShutterSpeedValue:         {tag: cIFDEXIF, name: "ShutterSpeedValue", id: ExifTagShutterSpeedValue},
	ExifTagApertureValue:             {tag: cIFDEXIF, name: "ApertureValue", id: ExifTagApertureValue},
	ExifTagBrightnessValue:           {tag: cIFDEXIF, name: "BrightnessValue", id: ExifTagBrightnessValue},
	ExifTagExposureBiasValue:         {tag: cIFDEXIF, name: "ExposureBiasValue", id: ExifTagExposureBiasValue},
	ExifTagMaxApertureValue:          {tag: cIFDEXIF, name: "MaxApertureValue", id: ExifTagMaxApertureValue},
	ExifTagSubjectDistance:           {tag: cIFDEXIF, name: "SubjectDistance", id: ExifTagSubjectDistance},
	ExifTagMeteringMode:              {tag: cIFDEXIF, name: "MeteringMode", id: ExifTagMeteringMode},
	ExifTagLightSource:               {tag: cIFDEXIF, name: "LightSource", id: ExifTagLightSource},
	ExifTagFlash:                     {tag: cIFDEXIF, name: "Flash", id: ExifTagFlash},
	ExifTagFocalLength:               {tag: cIFDEXIF, name: "FocalLength", id: ExifTagFocalLength},
	ExifTagSubjectArea:               {tag: cIFDEXIF, name: "SubjectArea", id: ExifTagSubjectArea},
	ExifTagMakerNote:                 {tag: cIFDEXIF, name: "MakerNote", id: ExifTagMakerNote},
	ExifTagUserComment:               {tag: cIFDEXIF, name: "UserComment", id: ExifTagUserComment},
	ExifTagSubsecTime:                {tag: cIFDEXIF, name: "SubsecTime", id: ExifTagSubsecTime},
	ExifTagSubsecTimeOriginal:        {tag: cIFDEXIF, name: "SubsecTimeOriginal", id: ExifTagSubsecTimeOriginal},
	ExifTagSubsecTimeDigitized:       {tag: cIFDEXIF, name: "SubsecTimeDigitized", id: ExifTagSubsecTimeDigitized},
	ExifTagFlashpixVersion:           {tag: cIFDEXIF, name: "FlashpixVersion", id: ExifTagFlashpixVersion},
	ExifTagColorSpace:                {tag: cIFDEXIF, name: "ColorSpace", id: ExifTagColorSpace},
	ExifTagPixelXDimension:           {tag: cIFDEXIF, name: "PixelXDimension", id: ExifTagPixelXDimension},
	ExifTagPixelYDimension:           {tag: cIFDEXIF, name: "PixelYDimension", id: ExifTagPixelYDimension},
	ExifTagRelatedSoundFile:          {tag: cIFDEXIF, name: "RelatedSoundFile", id: ExifTagRelatedSoundFile},
	ExifTagFlashEnergy:               {tag: cIFDEXIF, name: "FlashEnergy", id: ExifTagFlashEnergy},
	ExifTagSpatialFrequencyResponse:  {tag: cIFDEXIF, name: "SpatialFrequencyResponse", id: ExifTagSpatialFrequencyResponse},
	ExifTagFocalPlaneXResolution:     {tag: cIFDEXIF, name: "FocalPlaneXResolution", id: ExifTagFocalPlaneXResolution},
	ExifTagFocalPlaneYResolution:     {tag: cIFDEXIF, name: "FocalPlaneYResolution", id: ExifTagFocalPlaneYResolution},
	ExifTagFocalPlaneResolutionUnit:  {tag: cIFDEXIF, name: "FocalPlaneResolutionUnit", id: ExifTagFocalPlaneResolutionUnit},
	ExifTagSubjectLocation:           {tag: cIFDEXIF, name: "SubjectLocation", id: ExifTagSubjectLocation},
	ExifTagExposureIndex:             {tag: cIFDEXIF, name: "ExposureIndex", id: ExifTagExposureIndex},
	ExifTagSensingMethod:             {tag: cIFDEXIF, name: "SensingMethod", id: ExifTagSensingMethod},
	ExifTagFileSource:                {tag: cIFDEXIF, name: "FileSource", id: ExifTagFileSource},
	ExifTagSceneType:                 {tag: cIFDEXIF, name: "SceneType", id: ExifTagSceneType},
	ExifTagCFAPattern:                {tag: cIFDEXIF, name: "CFAPattern", id: ExifTagCFAPattern},
	ExifTagCustomRendered:            {tag: cIFDEXIF, name: "CustomRendered", id: ExifTagCustomRendered},
	ExifTagExposureMode:              {tag: cIFDEXIF, name: "ExposureMode", id: ExifTagExposureMode},
	ExifTagWhiteBalance:              {tag: cIFDEXIF, name: "WhiteBalance", id: ExifTagWhiteBalance},
	ExifTagDigitalZoomRatio:          {tag: cIFDEXIF, name: "DigitalZoomRatio", id: ExifTagDigitalZoomRatio},
	ExifTagFocalLengthIn35mmFilm:     {tag: cIFDEXIF, name: "FocalLengthIn35mmFilm", id: ExifTagFocalLengthIn35mmFilm},
	ExifTagSceneCaptureType:          {tag: cIFDEXIF, name: "SceneCaptureType", id: ExifTagSceneCaptureType},
	ExifTagGainControl:               {tag: cIFDEXIF, name: "GainControl", id: ExifTagGainControl},
	ExifTagContrast:                  {tag: cIFDEXIF, name: "Contrast", id: ExifTagContrast},
	ExifTagSaturation:                {tag: cIFDEXIF, name: "Saturation", id: ExifTagSaturation},
	ExifTagSharpness:                 {tag: cIFDEXIF, name: "Sharpness", id: ExifTagSharpness},
	ExifTagDeviceSettingDescription:  {tag: cIFDEXIF, name: "DeviceSettingDescription", id: ExifTagDeviceSettingDescription},
	ExifTagSubjectDistanceRange:      {tag: cIFDEXIF, name: "SubjectDistanceRange", id: ExifTagSubjectDistanceRange},
	ExifTagImageUniqueID:             {tag: cIFDEXIF, name: "ImageUniqueID", id: ExifTagImageUniqueID},
	ExifTagCameraOwnerName:           {tag: cIFDEXIF, name: "CameraOwnerName", id: ExifTagCameraOwnerName},
	ExifTagBodySerialNumber:          {tag: cIFDEXIF, name: "BodySerialNumber", id: ExifTagBodySerialNumber},
	ExifTagLensSpecification:         {tag: cIFDEXIF, name: "LensSpecification", id: ExifTagLensSpecification},
	ExifTagLensMake:                  {tag: cIFDEXIF, name: "LensMake", id: ExifTagLensMake},
	ExifTagLensModel:                 {tag: cIFDEXIF, name: "LensModel", id: ExifTagLensModel},
	ExifTagLensSerialNumber:          {tag: cIFDEXIF, name: "LensSerialNumber", id: ExifTagLensSerialNumber},

	// GPS tags
	ExifGpsTagGPSVersionID:         {tag: cIFDGPS, name: "GPSVersionID", id: ExifGpsTagGPSVersionID},
	ExifGpsTagGPSLatitudeRef:       {tag: cIFDGPS, name: "GPSLatitudeRef", id: ExifGpsTagGPSLatitudeRef},
	ExifGpsTagGPSLatitude:          {tag: cIFDGPS, name: "GPSLatitude", id: ExifGpsTagGPSLatitude},
	ExifGpsTagGPSLongitudeRef:      {tag: cIFDGPS, name: "GPSLongitudeRef", id: ExifGpsTagGPSLongitudeRef},
	ExifGpsTagGPSLongitude:         {tag: cIFDGPS, name: "GPSLongitude", id: ExifGpsTagGPSLongitude},
	ExifGpsTagGPSAltitudeRef:       {tag: cIFDGPS, name: "GPSAltitudeRef", id: ExifGpsTagGPSAltitudeRef},
	ExifGpsTagGPSAltitude:          {tag: cIFDGPS, name: "GPSAltitude", id: ExifGpsTagGPSAltitude},
	ExifGpsTagGPSTimestamp:         {tag: cIFDGPS, name: "GPSTimestamp", id: ExifGpsTagGPSTimestamp},
	ExifGpsTagGPSSatellites:        {tag: cIFDGPS, name: "GPSSatellites", id: ExifGpsTagGPSSatellites},
	ExifGpsTagGPSStatus:            {tag: cIFDGPS, name: "GPSStatus", id: ExifGpsTagGPSStatus},
	ExifGpsTagGPSMeasureMode:       {tag: cIFDGPS, name: "GPSMeasureMode", id: ExifGpsTagGPSMeasureMode},
	ExifGpsTagGPSDOP:               {tag: cIFDGPS, name: "GPSDOP", id: ExifGpsTagGPSDOP},
	ExifGpsTagGPSSpeedRef:          {tag: cIFDGPS, name: "GPSSpeedRef", id: ExifGpsTagGPSSpeedRef},
	ExifGpsTagGPSSpeed:             {tag: cIFDGPS, name: "GPSSpeed", id: ExifGpsTagGPSSpeed},
	ExifGpsTagGPSTrackRef:          {tag: cIFDGPS, name: "GPSTrackRef", id: ExifGpsTagGPSTrackRef},
	ExifGpsTagGPSTrack:             {tag: cIFDGPS, name: "GPSTrack", id: ExifGpsTagGPSTrack},
	ExifGpsTagGPSImgDirectionRef:   {tag: cIFDGPS, name: "GPSImgDirectionRef", id: ExifGpsTagGPSImgDirectionRef},
	ExifGpsTagGPSImgDirection:      {tag: cIFDGPS, name: "GPSImgDirection", id: ExifGpsTagGPSImgDirection},
	ExifGpsTagGPSMapDatum:          {tag: cIFDGPS, name: "GPSMapDatum", id: ExifGpsTagGPSMapDatum},
	ExifGpsTagGPSDestLatitudeRef:   {tag: cIFDGPS, name: "GPSDestLatitudeRef", id: ExifGpsTagGPSDestLatitudeRef},
	ExifGpsTagGPSDestLatitude:      {tag: cIFDGPS, name: "GPSDestLatitude", id: ExifGpsTagGPSDestLatitude},
	ExifGpsTagGPSDestLongitudeRef:  {tag: cIFDGPS, name: "GPSDestLongitudeRef", id: ExifGpsTagGPSDestLongitudeRef},
	ExifGpsTagGPSDestLongitude:     {tag: cIFDGPS, name: "GPSDestLongitude", id: ExifGpsTagGPSDestLongitude},
	ExifGpsTagGPSDestBearingRef:    {tag: cIFDGPS, name: "GPSDestBearingRef", id: ExifGpsTagGPSDestBearingRef},
	ExifGpsTagGPSDestBearing:       {tag: cIFDGPS, name: "GPSDestBearing", id: ExifGpsTagGPSDestBearing},
	ExifGpsTagGPSDestDistanceRef:   {tag: cIFDGPS, name: "GPSDestDistanceRef", id: ExifGpsTagGPSDestDistanceRef},
	ExifGpsTagGPSDestDistance:      {tag: cIFDGPS, name: "GPSDestDistance", id: ExifGpsTagGPSDestDistance},
	ExifGpsTagGPSProcessingMethod:  {tag: cIFDGPS, name: "GPSProcessingMethod", id: ExifGpsTagGPSProcessingMethod},
	ExifGpsTagGPSAreaInformation:   {tag: cIFDGPS, name: "GPSAreaInformation", id: ExifGpsTagGPSAreaInformation},
	ExifGpsTagGPSDateStamp:         {tag: cIFDGPS, name: "GPSDateStamp", id: ExifGpsTagGPSDateStamp},
	ExifGpsTagGPSDifferential:      {tag: cIFDGPS, name: "GPSDifferential", id: ExifGpsTagGPSDifferential},
	ExifGpsTagGPSHPositioningError: {tag: cIFDGPS, name: "GPSHPositioningError", id: ExifGpsTagGPSHPositioningError},

	// Microsoft Windows metadata. Non-standard, but ubiquitous
	ExifXpTagXPTitle:    {tag: cIFDZERO, name: "XPTitle", id: ExifXpTagXPTitle},
	ExifXpTagXPComment:  {tag: cIFDZERO, name: "XPComment", id: ExifXpTagXPComment},
	ExifXpTagXPAuthor:   {tag: cIFDZERO, name: "XPAuthor", id: ExifXpTagXPAuthor},
	ExifXpTagXPKeywords: {tag: cIFDZERO, name: "XPKeywords", id: ExifXpTagXPKeywords},
	ExifXpTagXPSubject:  {tag: cIFDZERO, name: "XPSubject", id: ExifXpTagXPSubject},
}

const (
	cExposureProgram      = 0x00010000
	cMeteringMode         = 0x00020000
	cLightSource          = 0x00030000
	cFlash                = 0x00040000
	cSensingMethod        = 0x00050000
	cSceneCaptureType     = 0x00060000
	cSceneType            = 0x00070000
	cCustomRendered       = 0x00080000
	cWhiteBalance         = 0x00090000
	cGainControl          = 0x000A0000
	cContrast             = 0x000B0000
	cSaturation           = 0x000C0000
	cSharpness            = 0x000D0000
	cSubjectDistanceRange = 0x000E0000
	cFileSource           = 0x000F0000
	cComponents           = 0x00100000
)

var aExifStringEnums = map[int]string{
	cExposureProgram + 0: "Not defined",
	cExposureProgram + 1: "Manual",
	cExposureProgram + 2: "Normal program",
	cExposureProgram + 3: "Aperture priority",
	cExposureProgram + 4: "Shutter priority",
	cExposureProgram + 5: "Creative program",
	cExposureProgram + 6: "Action program",
	cExposureProgram + 7: "Portrait mode",
	cExposureProgram + 8: "Landscape mode",

	cMeteringMode + 0:   "Unknown",
	cMeteringMode + 1:   "Average",
	cMeteringMode + 2:   "CenterWeightedAverage",
	cMeteringMode + 3:   "Spot",
	cMeteringMode + 4:   "MultiSpot",
	cMeteringMode + 5:   "Pattern",
	cMeteringMode + 6:   "Partial",
	cMeteringMode + 255: "Other",

	cLightSource + 0:   "Unknown",
	cLightSource + 1:   "Daylight",
	cLightSource + 2:   "Fluorescent",
	cLightSource + 3:   "Tungsten (incandescent light)",
	cLightSource + 4:   "Flash",
	cLightSource + 9:   "Fine weather",
	cLightSource + 10:  "Cloudy weather",
	cLightSource + 11:  "Shade",
	cLightSource + 12:  "Daylight fluorescent (D 5700 - 7100K)",
	cLightSource + 13:  "Day white fluorescent (N 4600 - 5400K)",
	cLightSource + 14:  "Cool white fluorescent (W 3900 - 4500K)",
	cLightSource + 15:  "White fluorescent (WW 3200 - 3700K)",
	cLightSource + 17:  "Standard light A",
	cLightSource + 18:  "Standard light B",
	cLightSource + 19:  "Standard light C",
	cLightSource + 20:  "D55",
	cLightSource + 21:  "D65",
	cLightSource + 22:  "D75",
	cLightSource + 23:  "D50",
	cLightSource + 24:  "ISO studio tungsten",
	cLightSource + 255: "Other",

	cFlash + 0x0000: "Flash did not fire",
	cFlash + 0x0001: "Flash fired",
	cFlash + 0x0005: "Strobe return light not detected",
	cFlash + 0x0007: "Strobe return light detected",
	cFlash + 0x0009: "Flash fired, compulsory flash mode",
	cFlash + 0x000D: "Flash fired, compulsory flash mode, return light not detected",
	cFlash + 0x000F: "Flash fired, compulsory flash mode, return light detected",
	cFlash + 0x0010: "Flash did not fire, compulsory flash mode",
	cFlash + 0x0018: "Flash did not fire, auto mode",
	cFlash + 0x0019: "Flash fired, auto mode",
	cFlash + 0x001D: "Flash fired, auto mode, return light not detected",
	cFlash + 0x001F: "Flash fired, auto mode, return light detected",
	cFlash + 0x0020: "No flash function",
	cFlash + 0x0041: "Flash fired, red-eye reduction mode",
	cFlash + 0x0045: "Flash fired, red-eye reduction mode, return light not detected",
	cFlash + 0x0047: "Flash fired, red-eye reduction mode, return light detected",
	cFlash + 0x0049: "Flash fired, compulsory flash mode, red-eye reduction mode",
	cFlash + 0x004D: "Flash fired, compulsory flash mode, red-eye reduction mode, return light not detected",
	cFlash + 0x004F: "Flash fired, compulsory flash mode, red-eye reduction mode, return light detected",
	cFlash + 0x0059: "Flash fired, auto mode, red-eye reduction mode",
	cFlash + 0x005D: "Flash fired, auto mode, return light not detected, red-eye reduction mode",
	cFlash + 0x005F: "Flash fired, auto mode, return light detected, red-eye reduction mode",

	cSensingMethod + 1: "Not defined",
	cSensingMethod + 2: "One-chip color area sensor",
	cSensingMethod + 3: "Two-chip color area sensor",
	cSensingMethod + 4: "Three-chip color area sensor",
	cSensingMethod + 5: "Color sequential area sensor",
	cSensingMethod + 7: "Trilinear sensor",
	cSensingMethod + 8: "Color sequential linear sensor",

	cSceneCaptureType + 0: "Standard",
	cSceneCaptureType + 1: "Landscape",
	cSceneCaptureType + 2: "Portrait",
	cSceneCaptureType + 3: "Night scene",

	cSceneType + 1: "Directly photographed",

	cCustomRendered + 0: "Normal process",
	cCustomRendered + 1: "Custom process",

	cWhiteBalance + 0: "Auto white balance",
	cWhiteBalance + 1: "Manual white balance",

	cGainControl + 0: "None",
	cGainControl + 1: "Low gain up",
	cGainControl + 2: "High gain up",
	cGainControl + 3: "Low gain down",
	cGainControl + 4: "High gain down",

	cContrast + 0: "Normal",
	cContrast + 1: "Soft",
	cContrast + 2: "Hard",

	cSaturation + 0: "Normal",
	cSaturation + 1: "Low saturation",
	cSaturation + 2: "High saturation",

	cSharpness + 0: "Normal",
	cSharpness + 1: "Soft",
	cSharpness + 2: "Hard",

	cSubjectDistanceRange + 0: "Unknown",
	cSubjectDistanceRange + 1: "Macro",
	cSubjectDistanceRange + 2: "Close view",
	cSubjectDistanceRange + 3: "Distant view",

	cFileSource + 3: "DSC",

	cComponents + 0: "",
	cComponents + 1: "Y",
	cComponents + 2: "Cb",
	cComponents + 3: "Cr",
	cComponents + 4: "R",
	cComponents + 5: "G",
	cComponents + 6: "B",
}
