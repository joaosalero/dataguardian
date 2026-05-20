package analysis

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"dataguardian/backend-go/internal/db"
)

var pdfLiteralMetadataPattern = regexp.MustCompile(`/([A-Za-z]+)\s*\(([^)]*)\)`)

var ErrUnsupportedFileType = errors.New("unsupported file type")

// CleanResult contains sanitized bytes produced without executing or rendering content.
type CleanResult struct {
	Content             []byte
	RemovedMetadataKeys []string
}

// ExtractFileMetadata reads a small, passive subset of PDF and EXIF metadata from raw bytes.
func ExtractFileMetadata(content []byte, mimeType string) []db.MetadataEntry {
	entries := make([]db.MetadataEntry, 0)
	switch mimeType {
	case "application/pdf":
		entries = append(entries, extractPDFMetadata(content)...)
	case "image/jpeg":
		entries = append(entries, extractJPEGMetadata(content)...)
	}
	return entries
}

func extractPDFMetadata(content []byte) []db.MetadataEntry {
	values := pdfLiteralMetadata(content)
	entries := make([]db.MetadataEntry, 0, 4)
	if value := values["Producer"]; value != "" {
		entries = append(entries, fileMetadataEntry("producer", value, db.MetadataCategoryPDF, db.MetadataSensitivityPotentiallySensitive, "pdf_info"))
	}
	if value := values["Author"]; value != "" {
		entries = append(entries, fileMetadataEntry("author", value, db.MetadataCategoryPDF, db.MetadataSensitivityPotentiallySensitive, "pdf_info"))
	}
	if value := values["CreationDate"]; value != "" {
		entries = append(entries, fileMetadataEntry("creation_date", value, db.MetadataCategoryPDF, db.MetadataSensitivityPotentiallySensitive, "pdf_info"))
	}
	if hasEmbeddedPDFObjects(content) {
		entries = append(entries, fileMetadataEntry("embedded_objects", true, db.MetadataCategoryPDF, db.MetadataSensitivityPotentiallySensitive, "pdf_structure"))
	}
	return entries
}

func pdfLiteralMetadata(content []byte) map[string]string {
	values := map[string]string{}
	for _, match := range pdfLiteralMetadataPattern.FindAllSubmatch(content, -1) {
		key := string(match[1])
		if key != "Producer" && key != "Author" && key != "CreationDate" {
			continue
		}
		if _, exists := values[key]; exists {
			continue
		}
		if value := strings.TrimSpace(unescapePDFLiteral(match[2])); value != "" {
			values[key] = value
		}
	}
	return values
}

func unescapePDFLiteral(value []byte) string {
	replacer := strings.NewReplacer(`\\`, `\`, `\(`, `(`, `\)`, `)`, `\n`, "\n", `\r`, "\r", `\t`, "\t")
	return replacer.Replace(string(value))
}

func hasEmbeddedPDFObjects(content []byte) bool {
	return bytes.Contains(content, []byte("/EmbeddedFile")) || bytes.Contains(content, []byte("/Filespec"))
}

func extractJPEGMetadata(content []byte) []db.MetadataEntry {
	exif, ok := firstJPEGAPP1EXIF(content)
	if !ok {
		return nil
	}
	parsed := parseEXIF(exif)
	entries := make([]db.MetadataEntry, 0, 3)
	if parsed.CameraModel != "" {
		entries = append(entries, fileMetadataEntry("camera_model", parsed.CameraModel, db.MetadataCategoryEXIF, db.MetadataSensitivityPotentiallySensitive, "exif"))
	}
	if parsed.DateTime != "" {
		entries = append(entries, fileMetadataEntry("datetime", parsed.DateTime, db.MetadataCategoryEXIF, db.MetadataSensitivityPotentiallySensitive, "exif"))
	}
	if parsed.GPS != "" {
		entries = append(entries, fileMetadataEntry("gps", parsed.GPS, db.MetadataCategoryEXIF, db.MetadataSensitivitySensitive, "exif_gps"))
	}
	return entries
}

func fileMetadataEntry(key string, value any, category db.MetadataCategory, sensitivity db.MetadataSensitivity, source string) db.MetadataEntry {
	return db.MetadataEntry{
		Key:         key,
		Value:       value,
		Category:    category,
		Sensitivity: sensitivity,
		Source:      source,
		Confidence:  db.MetadataConfidenceMedium,
	}
}

func metadataFindings(entries []db.MetadataEntry) []db.Finding {
	findings := make([]db.Finding, 0)
	for _, entry := range entries {
		switch entry.Key {
		case "gps":
			findings = append(findings, metadataFinding(
				"METADATA_GPS_EXPOSED",
				"GPS metadata exposed",
				"The file contains GPS metadata that may reveal a location.",
				db.SeverityHigh,
				entry.Key,
			))
		case "author", "producer", "camera_model":
			findings = append(findings, metadataFinding(
				"METADATA_AUTHOR_PRESENT",
				"Author or tool metadata present",
				"The file contains authoring or device/tool metadata.",
				db.SeverityMedium,
				entry.Key,
			))
		case "embedded_objects":
			findings = append(findings, metadataFinding(
				"METADATA_SUSPICIOUS_PRESENT",
				"Suspicious metadata present",
				"The file metadata indicates embedded objects are present.",
				db.SeverityMedium,
				entry.Key,
			))
		}
	}
	return findings
}

func metadataFinding(code string, title string, description string, severity db.Severity, metadataKey string) db.Finding {
	return db.Finding{
		Type:        db.FindingTypeMetadata,
		Code:        code,
		Title:       title,
		Description: description,
		Severity:    severity,
		Evidence: db.FindingEvidence{
			Source:      db.FindingEvidenceSourceMetadata,
			MetadataKey: stringPtr(metadataKey),
			RuleID:      code,
		},
	}
}

// SanitizeFile removes metadata-only containers from supported file bytes.
func SanitizeFile(content []byte, mimeType string) (CleanResult, error) {
	switch mimeType {
	case "application/pdf":
		return sanitizePDF(content), nil
	case "image/jpeg":
		return sanitizeJPEG(content), nil
	default:
		return CleanResult{}, ErrUnsupportedFileType
	}
}

func sanitizePDF(content []byte) CleanResult {
	cleaned := append([]byte(nil), content...)
	removed := make([]string, 0)
	for _, key := range []string{"Author", "Producer", "CreationDate"} {
		pattern := regexp.MustCompile(`/` + key + `\s*\([^)]*\)`)
		if pattern.Match(cleaned) {
			removed = append(removed, strings.ToLower(camelToSnake(key)))
			cleaned = pattern.ReplaceAll(cleaned, nil)
		}
	}
	return CleanResult{Content: cleaned, RemovedMetadataKeys: removed}
}

func sanitizeJPEG(content []byte) CleanResult {
	if len(content) < 4 || content[0] != 0xff || content[1] != 0xd8 {
		return CleanResult{Content: append([]byte(nil), content...)}
	}
	cleaned := []byte{0xff, 0xd8}
	removed := make([]string, 0)
	pos := 2
	for pos+4 <= len(content) {
		if content[pos] != 0xff {
			cleaned = append(cleaned, content[pos:]...)
			return CleanResult{Content: cleaned, RemovedMetadataKeys: removed}
		}
		marker := content[pos+1]
		if marker == 0xda || marker == 0xd9 {
			cleaned = append(cleaned, content[pos:]...)
			return CleanResult{Content: cleaned, RemovedMetadataKeys: removed}
		}
		segmentLength := int(binary.BigEndian.Uint16(content[pos+2 : pos+4]))
		end := pos + 2 + segmentLength
		if segmentLength < 2 || end > len(content) {
			cleaned = append(cleaned, content[pos:]...)
			return CleanResult{Content: cleaned, RemovedMetadataKeys: removed}
		}
		segment := content[pos:end]
		if marker == 0xe1 && bytes.HasPrefix(content[pos+4:end], []byte("Exif\x00\x00")) {
			removed = append(removed, "exif")
			pos = end
			continue
		}
		cleaned = append(cleaned, segment...)
		pos = end
	}
	cleaned = append(cleaned, content[pos:]...)
	return CleanResult{Content: cleaned, RemovedMetadataKeys: removed}
}

func camelToSnake(value string) string {
	var out strings.Builder
	for index, r := range value {
		if index > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte('_')
		}
		out.WriteRune(r)
	}
	return out.String()
}

type exifMetadata struct {
	CameraModel string
	DateTime    string
	GPS         string
}

func firstJPEGAPP1EXIF(content []byte) ([]byte, bool) {
	if len(content) < 4 || content[0] != 0xff || content[1] != 0xd8 {
		return nil, false
	}
	for pos := 2; pos+4 <= len(content); {
		if content[pos] != 0xff {
			return nil, false
		}
		marker := content[pos+1]
		if marker == 0xda || marker == 0xd9 {
			return nil, false
		}
		segmentLength := int(binary.BigEndian.Uint16(content[pos+2 : pos+4]))
		end := pos + 2 + segmentLength
		if segmentLength < 2 || end > len(content) {
			return nil, false
		}
		payload := content[pos+4 : end]
		if marker == 0xe1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			return payload[6:], true
		}
		pos = end
	}
	return nil, false
}

func parseEXIF(tiff []byte) exifMetadata {
	if len(tiff) < 8 {
		return exifMetadata{}
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return exifMetadata{}
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return exifMetadata{}
	}
	ifdOffset := int(order.Uint32(tiff[4:8]))
	tags := parseIFD(tiff, ifdOffset, order)
	metadata := exifMetadata{
		CameraModel: exifASCII(tiff, order, tags[0x0110]),
		DateTime:    exifASCII(tiff, order, tags[0x0132]),
	}
	if gpsEntry, ok := tags[0x8825]; ok && len(gpsEntry.value) == 4 {
		gpsTags := parseIFD(tiff, int(order.Uint32(gpsEntry.value)), order)
		metadata.GPS = exifGPS(tiff, order, gpsTags)
	}
	return metadata
}

type exifEntry struct {
	fieldType uint16
	count     uint32
	value     []byte
}

func parseIFD(tiff []byte, offset int, order binary.ByteOrder) map[uint16]exifEntry {
	entries := map[uint16]exifEntry{}
	if offset < 0 || offset+2 > len(tiff) {
		return entries
	}
	count := int(order.Uint16(tiff[offset : offset+2]))
	pos := offset + 2
	for i := 0; i < count && pos+12 <= len(tiff); i++ {
		tag := order.Uint16(tiff[pos : pos+2])
		fieldType := order.Uint16(tiff[pos+2 : pos+4])
		valueCount := order.Uint32(tiff[pos+4 : pos+8])
		valueSize := exifTypeSize(fieldType) * int(valueCount)
		raw := tiff[pos+8 : pos+12]
		value := raw
		if valueSize > 4 {
			valueOffset := int(order.Uint32(raw))
			if valueOffset < 0 || valueOffset+valueSize > len(tiff) {
				pos += 12
				continue
			}
			value = tiff[valueOffset : valueOffset+valueSize]
		} else {
			value = raw[:valueSize]
		}
		entries[tag] = exifEntry{fieldType: fieldType, count: valueCount, value: append([]byte(nil), value...)}
		pos += 12
	}
	return entries
}

func exifTypeSize(fieldType uint16) int {
	switch fieldType {
	case 1, 2, 7:
		return 1
	case 3:
		return 2
	case 4, 9:
		return 4
	case 5, 10:
		return 8
	default:
		return 0
	}
}

func exifASCII(tiff []byte, order binary.ByteOrder, entry exifEntry) string {
	if entry.fieldType != 2 || entry.count == 0 {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(string(entry.value)), "\x00")
}

func exifGPS(tiff []byte, order binary.ByteOrder, tags map[uint16]exifEntry) string {
	lat := exifRationalTriplet(order, tags[2])
	lon := exifRationalTriplet(order, tags[4])
	if math.IsNaN(lat) || math.IsNaN(lon) {
		return ""
	}
	if strings.HasPrefix(exifASCII(tiff, order, tags[1]), "S") {
		lat = -lat
	}
	if strings.HasPrefix(exifASCII(tiff, order, tags[3]), "W") {
		lon = -lon
	}
	return strings.TrimRight(strings.TrimRight(formatFloat(lat), "0"), ".") + "," + strings.TrimRight(strings.TrimRight(formatFloat(lon), "0"), ".")
}

func exifRationalTriplet(order binary.ByteOrder, entry exifEntry) float64 {
	if entry.fieldType != 5 || entry.count < 3 || len(entry.value) < 24 {
		return math.NaN()
	}
	deg := rational(order, entry.value[0:8])
	min := rational(order, entry.value[8:16])
	sec := rational(order, entry.value[16:24])
	if math.IsNaN(deg) || math.IsNaN(min) || math.IsNaN(sec) {
		return math.NaN()
	}
	return deg + min/60 + sec/3600
}

func rational(order binary.ByteOrder, value []byte) float64 {
	denominator := order.Uint32(value[4:8])
	if denominator == 0 {
		return math.NaN()
	}
	return float64(order.Uint32(value[0:4])) / float64(denominator)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}
