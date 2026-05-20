package analysis

import (
	"bytes"
	"encoding/binary"
	"testing"

	"dataguardian/backend-go/internal/db"
)

func TestExtractPDFMetadataClassifiesAndFindsSensitiveFields(t *testing.T) {
	content := []byte("%PDF-1.7\n<< /Author (Alice) /Producer (DataTool) /CreationDate (D:20260505120000Z) /EmbeddedFile 7 0 R >>")

	result := AnalyzeFile(content, "application/pdf")

	entries := entriesByKey(result.MetadataEntries)
	if entries["author"].Sensitivity != db.MetadataSensitivityPotentiallySensitive || entries["author"].Category != db.MetadataCategoryPDF {
		t.Fatalf("expected classified author metadata, got %#v", entries["author"])
	}
	if entries["embedded_objects"].Value != true {
		t.Fatalf("expected embedded object metadata, got %#v", entries["embedded_objects"])
	}
	codes := findingCodes(result.Findings)
	if !codes["METADATA_AUTHOR_PRESENT"] || !codes["METADATA_SUSPICIOUS_PRESENT"] {
		t.Fatalf("expected metadata findings, got %#v", result.Findings)
	}
}

func TestExtractJPEGEXIFMetadataIncludesGPS(t *testing.T) {
	content := jpegWithEXIF(t)

	result := AnalyzeFile(content, "image/jpeg")

	entries := entriesByKey(result.MetadataEntries)
	if entries["camera_model"].Value != "TestCam" {
		t.Fatalf("expected camera model metadata, got %#v", entries["camera_model"])
	}
	if entries["gps"].Sensitivity != db.MetadataSensitivitySensitive {
		t.Fatalf("expected GPS to be sensitive, got %#v", entries["gps"])
	}
	codes := findingCodes(result.Findings)
	if !codes["METADATA_GPS_EXPOSED"] || !codes["METADATA_AUTHOR_PRESENT"] {
		t.Fatalf("expected GPS and author/tool findings, got %#v", result.Findings)
	}
}

func TestSanitizePDFRemovesNonEssentialMetadata(t *testing.T) {
	content := []byte("%PDF-1.7\n<< /Author (Alice) /Producer (DataTool) /CreationDate (D:20260505120000Z) >>")

	clean, err := SanitizeFile(content, "application/pdf")
	if err != nil {
		t.Fatalf("SanitizeFile returned error: %v", err)
	}
	if bytes.Contains(clean.Content, []byte("/Author")) || bytes.Contains(clean.Content, []byte("/Producer")) {
		t.Fatalf("expected PDF metadata removed, got %s", clean.Content)
	}
	if len(clean.RemovedMetadataKeys) != 3 {
		t.Fatalf("expected removed PDF metadata keys, got %#v", clean.RemovedMetadataKeys)
	}
}

func TestSanitizeJPEGRemovesEXIFSegment(t *testing.T) {
	content := jpegWithEXIF(t)

	clean, err := SanitizeFile(content, "image/jpeg")
	if err != nil {
		t.Fatalf("SanitizeFile returned error: %v", err)
	}
	if bytes.Contains(clean.Content, []byte("Exif\x00\x00")) {
		t.Fatalf("expected EXIF segment removed")
	}
	if len(clean.RemovedMetadataKeys) != 1 || clean.RemovedMetadataKeys[0] != "exif" {
		t.Fatalf("expected EXIF removed key, got %#v", clean.RemovedMetadataKeys)
	}
}

func entriesByKey(entries []db.MetadataEntry) map[string]db.MetadataEntry {
	byKey := map[string]db.MetadataEntry{}
	for _, entry := range entries {
		byKey[entry.Key] = entry
	}
	return byKey
}

func findingCodes(findings []db.Finding) map[string]bool {
	codes := map[string]bool{}
	for _, finding := range findings {
		codes[finding.Code] = true
	}
	return codes
}

func jpegWithEXIF(t *testing.T) []byte {
	t.Helper()
	tiff := buildLittleEndianEXIF()
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segmentLength := len(payload) + 2
	content := []byte{0xff, 0xd8, 0xff, 0xe1, byte(segmentLength >> 8), byte(segmentLength)}
	content = append(content, payload...)
	content = append(content, 0xff, 0xda, 0x00, 0x0c, 0x00, 0xff, 0xd9)
	return content
}

func buildLittleEndianEXIF() []byte {
	tiff := make([]byte, 228)
	copy(tiff[0:2], []byte("II"))
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)
	binary.LittleEndian.PutUint16(tiff[8:10], 3)
	putIFDEntry(tiff[10:22], 0x0110, 2, 8, 80)
	putIFDEntry(tiff[22:34], 0x0132, 2, 20, 96)
	putIFDEntry(tiff[34:46], 0x8825, 4, 1, 128)
	copy(tiff[80:], []byte("TestCam\x00"))
	copy(tiff[96:], []byte("2026:05:05 12:00:00\x00"))
	binary.LittleEndian.PutUint16(tiff[128:130], 4)
	putIFDEntry(tiff[130:142], 1, 2, 2, uint32('N'))
	putIFDEntry(tiff[142:154], 2, 5, 3, 180)
	putIFDEntry(tiff[154:166], 3, 2, 2, uint32('W'))
	putIFDEntry(tiff[166:178], 4, 5, 3, 204)
	putRational(tiff[180:188], 40, 1)
	putRational(tiff[188:196], 30, 1)
	putRational(tiff[196:204], 0, 1)
	putRational(tiff[204:212], 3, 1)
	putRational(tiff[212:220], 10, 1)
	putRational(tiff[220:228], 0, 1)
	return tiff
}

func putIFDEntry(dst []byte, tag uint16, fieldType uint16, count uint32, value uint32) {
	binary.LittleEndian.PutUint16(dst[0:2], tag)
	binary.LittleEndian.PutUint16(dst[2:4], fieldType)
	binary.LittleEndian.PutUint32(dst[4:8], count)
	binary.LittleEndian.PutUint32(dst[8:12], value)
}

func putRational(dst []byte, numerator uint32, denominator uint32) {
	binary.LittleEndian.PutUint32(dst[0:4], numerator)
	binary.LittleEndian.PutUint32(dst[4:8], denominator)
}
