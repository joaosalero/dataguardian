package httpapi

import "dataguardian/backend-go/internal/db"

func fileMetadata(analysisID int64, file db.File, extracted []db.MetadataEntry) db.Metadata {
	entries := []db.MetadataEntry{
		metadataEntry("filename", file.OriginalFilename),
		metadataEntry("mime_type", file.MimeType),
		metadataEntry("size_bytes", file.SizeBytes),
		metadataEntry("checksum_sha256", file.ChecksumSHA256),
	}
	entries = append(entries, extracted...)
	return db.Metadata{
		AnalysisID: analysisID,
		SourceType: db.MetadataSourceTypeFile,
		Entries:    entries,
	}
}

func metadataEntry(key string, value any) db.MetadataEntry {
	return db.MetadataEntry{
		Key:         key,
		Value:       value,
		Category:    db.MetadataCategoryGeneric,
		Sensitivity: db.MetadataSensitivityNonSensitive,
		Source:      "file_upload",
		Confidence:  db.MetadataConfidenceHigh,
	}
}

func buildFileAnalysisResponse(
	analysis db.Analysis,
	file db.File,
	metadata db.Metadata,
	findings []db.Finding,
	riskScore db.RiskScore,
	cleanFile *db.CleanFile,
) AnalysisResponse {
	response := AnalysisResponse{
		AnalysisID:    analysis.ID,
		ProjectID:     analysis.ProjectID,
		InputType:     analysis.InputType,
		Status:        analysis.Status,
		Summary:       analysis.Summary,
		StartedAt:     analysis.StartedAt,
		CompletedAt:   analysis.CompletedAt,
		FailureReason: analysis.FailureReason,
		File: &AnalysisFileReference{
			ID:               file.ID,
			OriginalFilename: file.OriginalFilename,
			MimeType:         file.MimeType,
			SizeBytes:        file.SizeBytes,
			ChecksumSHA256:   file.ChecksumSHA256,
			Extension:        file.Extension,
		},
		Findings:  analysisFindings(findings),
		Metadata:  analysisMetadata(metadata),
		RiskScore: analysisRiskScore(riskScore),
	}
	if cleanFile != nil {
		response.CleanFile = &AnalysisCleanFileReference{
			ID:                  cleanFile.ID,
			Filename:            cleanFile.Filename,
			MimeType:            cleanFile.MimeType,
			SizeBytes:           cleanFile.SizeBytes,
			ChecksumSHA256:      cleanFile.ChecksumSHA256,
			CleaningStatus:      cleanFile.CleaningStatus,
			RemovedMetadataKeys: cleanFile.RemovedMetadataKeys,
		}
	}
	return response
}

func buildURLAnalysisResponse(
	analysis db.Analysis,
	target db.URLTarget,
	metadata db.Metadata,
	findings []db.Finding,
	riskScore db.RiskScore,
) AnalysisResponse {
	return AnalysisResponse{
		AnalysisID:    analysis.ID,
		ProjectID:     analysis.ProjectID,
		InputType:     analysis.InputType,
		Status:        analysis.Status,
		Summary:       analysis.Summary,
		StartedAt:     analysis.StartedAt,
		CompletedAt:   analysis.CompletedAt,
		FailureReason: analysis.FailureReason,
		URLTarget: &AnalysisURLTarget{
			ID:                 target.ID,
			OriginalURL:        target.OriginalURL,
			FinalURL:           target.FinalURL,
			RedirectCount:      target.RedirectCount,
			RedirectChain:      target.RedirectChain,
			UsesHTTPS:          target.UsesHTTPS,
			Host:               target.Host,
			ContentType:        target.ContentType,
			ContentLengthBytes: target.ContentLengthBytes,
			HTTPStatusCode:     target.HTTPStatusCode,
			FetchStatus:        target.FetchStatus,
			FailureReason:      target.FailureReason,
		},
		Findings:  analysisFindings(findings),
		Metadata:  analysisMetadata(metadata),
		RiskScore: analysisRiskScore(riskScore),
		CleanFile: nil,
	}
}

func analysisFindings(findings []db.Finding) []AnalysisFinding {
	response := make([]AnalysisFinding, 0, len(findings))
	for _, finding := range findings {
		explanation, recommendation := explainFinding(finding)
		if finding.Recommendation != nil {
			recommendation = finding.Recommendation
		}
		response = append(response, AnalysisFinding{
			ID:             finding.ID,
			Type:           finding.Type,
			Code:           finding.Code,
			Title:          finding.Title,
			Description:    finding.Description,
			Severity:       finding.Severity,
			Evidence:       finding.Evidence,
			Explanation:    explanation,
			Recommendation: recommendation,
		})
	}
	return response
}

func explainFinding(finding db.Finding) (string, *string) {
	if explanationProvider == nil {
		return "", finding.Recommendation
	}
	// Explanations are best-effort response decoration; failures never block analysis results.
	result, err := explanationProvider.ExplainFinding(finding)
	if err != nil {
		return "", finding.Recommendation
	}
	return result.Explanation, stringPtrOrNil(result.Recommendation)
}

func analysisMetadata(metadata db.Metadata) AnalysisMetadata {
	return AnalysisMetadata{
		ID:         metadata.ID,
		SourceType: metadata.SourceType,
		Entries:    metadata.Entries,
	}
}

func analysisRiskScore(score db.RiskScore) AnalysisRiskScore {
	return AnalysisRiskScore{
		Score:   score.Score,
		Level:   score.Level,
		Drivers: score.Drivers,
	}
}
