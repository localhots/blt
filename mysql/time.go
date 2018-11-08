package mysql

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// DecodeTimestamp2 ...
// Implementation borrowed from https://github.com/siddontang/go-mysql/
func DecodeTimestamp2(data []byte, dec uint16) (string, int) {
	//get timestamp binary length
	n := int(4 + (dec+1)/2)
	sec := int64(binary.BigEndian.Uint32(data[0:4]))
	usec := int64(0)
	switch dec {
	case 1, 2:
		usec = int64(data[4]) * 10000
	case 3, 4:
		usec = int64(binary.BigEndian.Uint16(data[4:])) * 100
	case 5, 6:
		usec = int64(DecodeVarLen64BigEndian(data[4:7]))
	}

	if sec == 0 {
		return formatZeroTime(int(usec), int(dec)), n
	}

	return FracTime{time.Unix(sec, usec*1000), int(dec)}.String(), n
}

// DecodeDatetime2 ...
// Implementation borrowed from https://github.com/siddontang/go-mysql/
func DecodeDatetime2(data []byte, dec uint16) (string, int) {
	const offset int64 = 0x8000000000
	//get datetime binary length
	n := int(5 + (dec+1)/2)

	intPart := int64(DecodeVarLen64BigEndian(data[0:5])) - offset
	var frac int64

	switch dec {
	case 1, 2:
		frac = int64(data[5]) * 10000
	case 3, 4:
		frac = int64(binary.BigEndian.Uint16(data[5:7])) * 100
	case 5, 6:
		frac = int64(DecodeVarLen64BigEndian(data[5:8]))
	}

	if intPart == 0 {
		return formatZeroTime(int(frac), int(dec)), n
	}

	tmp := intPart<<24 + frac
	//handle sign???
	if tmp < 0 {
		tmp = -tmp
	}

	// var secPart int64 = tmp % (1 << 24)
	ymdhms := tmp >> 24

	ymd := ymdhms >> 17
	ym := ymd >> 5
	hms := ymdhms % (1 << 17)

	day := int(ymd % (1 << 5))
	month := int(ym % 13)
	year := int(ym / 13)

	second := int(hms % (1 << 6))
	minute := int((hms >> 6) % (1 << 6))
	hour := int((hms >> 12))

	return FracTime{time.Date(year, time.Month(month), day, hour, minute, second, int(frac*1000), time.UTC), int(dec)}.String(), n
}

// DecodeTime2 ...
// Implementation borrowed from https://github.com/siddontang/go-mysql/
func DecodeTime2(data []byte, dec uint16) (string, int) {
	const offset int64 = 0x800000000000
	const intOffset int64 = 0x800000
	//time  binary length
	n := int(3 + (dec+1)/2)

	tmp := int64(0)
	intPart := int64(0)
	frac := int64(0)
	switch dec {
	case 1:
	case 2:
		intPart = int64(DecodeVarLen64BigEndian(data[0:3])) - intOffset
		frac = int64(data[3])
		if intPart < 0 && frac > 0 {
			intPart++     /* Shift to the next integer value */
			frac -= 0x100 /* -(0x100 - frac) */
		}
		tmp = intPart<<24 + frac*10000
	case 3:
	case 4:
		intPart = int64(DecodeVarLen64BigEndian(data[0:3])) - intOffset
		frac = int64(binary.BigEndian.Uint16(data[3:5]))
		if intPart < 0 && frac > 0 {
			/*
			   Fix reverse fractional part order: "0x10000 - frac".
			   See comments for FSP=1 and FSP=2 above.
			*/
			intPart++       /* Shift to the next integer value */
			frac -= 0x10000 /* -(0x10000-frac) */
		}
		tmp = intPart<<24 + frac*100

	case 5:
	case 6:
		tmp = int64(DecodeVarLen64BigEndian(data[0:6])) - offset
	default:
		intPart = int64(DecodeVarLen64BigEndian(data[0:3])) - intOffset
		tmp = intPart << 24
	}

	if intPart == 0 {
		return "00:00:00", n
	}

	hms := int64(0)
	sign := ""
	if tmp < 0 {
		tmp = -tmp
		sign = "-"
	}

	hms = tmp >> 24

	hour := (hms >> 12) % (1 << 10) /* 10 bits starting at 12th */
	minute := (hms >> 6) % (1 << 6) /* 6 bits starting at 6th   */
	second := hms % (1 << 6)        /* 6 bits starting at 0th   */
	secPart := tmp % (1 << 24)

	if secPart != 0 {
		return fmt.Sprintf("%s%02d:%02d:%02d.%06d", sign, hour, minute, second, secPart), n
	}

	return fmt.Sprintf("%s%02d:%02d:%02d", sign, hour, minute, second), n
}

var (
	fracTimeFormat []string
)

// FracTime is a help structure wrapping Golang Time.
type FracTime struct {
	time.Time

	// Dec must in [0, 6]
	Dec int
}

func (t FracTime) String() string {
	return t.Format(fracTimeFormat[t.Dec])
}

func formatZeroTime(frac int, dec int) string {
	if dec == 0 {
		return "0000-00-00 00:00:00"
	}

	s := fmt.Sprintf("0000-00-00 00:00:00.%06d", frac)

	// dec must < 6, if frac is 924000, but dec is 3, we must output 924 here.
	return s[0 : len(s)-(6-dec)]
}

func init() {
	fracTimeFormat = make([]string, 7)
	fracTimeFormat[0] = "2006-01-02 15:04:05"

	for i := 1; i <= 6; i++ {
		fracTimeFormat[i] = fmt.Sprintf("2006-01-02 15:04:05.%s", strings.Repeat("0", i))
	}
}
