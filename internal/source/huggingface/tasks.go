package huggingface

import "strings"

// TaskGroups are the ~10 buckets the ~40 official pipeline tags collapse into.
// Stored as categoryPath "hf/<group>" — parallel to, never mixed with, the
// GitHub domain taxonomy.
var TaskGroups = []string{
	"text-gen", "multimodal", "image-gen", "video", "audio",
	"embedding", "vision", "nlp", "rl", "other",
}

// taskOfPipeline maps an official pipeline_tag to its group.
var taskOfPipeline = map[string]string{
	// text generation
	"text-generation":      "text-gen",
	"text2text-generation": "text-gen",
	"fill-mask":            "text-gen",
	// multimodal understanding / any-to-any
	"image-text-to-text":          "multimodal",
	"visual-question-answering":   "multimodal",
	"document-question-answering": "multimodal",
	"video-text-to-text":          "multimodal",
	"any-to-any":                  "multimodal",
	"image-to-text":               "multimodal",
	"visual-document-retrieval":   "multimodal",
	// image generation
	"text-to-image":                  "image-gen",
	"image-to-image":                 "image-gen",
	"unconditional-image-generation": "image-gen",
	"text-to-3d":                     "image-gen",
	"image-to-3d":                    "image-gen",
	// video
	"text-to-video":  "video",
	"image-to-video": "video",
	"video-to-video": "video",
	// audio / speech
	"automatic-speech-recognition": "audio",
	"text-to-speech":               "audio",
	"text-to-audio":                "audio",
	"audio-to-audio":               "audio",
	"audio-classification":         "audio",
	"voice-activity-detection":     "audio",
	"audio-text-to-text":           "audio",
	// embeddings / retrieval
	"feature-extraction":  "embedding",
	"sentence-similarity": "embedding",
	// vision understanding
	"image-classification":           "vision",
	"object-detection":               "vision",
	"image-segmentation":             "vision",
	"depth-estimation":               "vision",
	"zero-shot-image-classification": "vision",
	"zero-shot-object-detection":     "vision",
	"keypoint-detection":             "vision",
	"video-classification":           "vision",
	"image-feature-extraction":       "vision",
	"mask-generation":                "vision",
	// classic NLP
	"text-classification":      "nlp",
	"token-classification":     "nlp",
	"translation":              "nlp",
	"summarization":            "nlp",
	"question-answering":       "nlp",
	"zero-shot-classification": "nlp",
	"table-question-answering": "nlp",
	"text-ranking":             "nlp",
	// RL / robotics
	"reinforcement-learning": "rl",
	"robotics":               "rl",
	// tabular & misc sink to other via fallback
}

// TaskGroup returns the group for a pipeline tag ("other" when unknown/empty).
func TaskGroup(pipelineTag string) string {
	if g, ok := taskOfPipeline[strings.ToLower(strings.TrimSpace(pipelineTag))]; ok {
		return g
	}
	return "other"
}

// quantFormats are packaging tags local-deployment users filter by.
var quantFormats = []string{"gguf", "awq", "gptq", "mlx", "onnx", "safetensors", "exl2"}

// QuantFormats extracts the recognized packaging/quantization formats from tags.
func QuantFormats(tags []string) []string {
	seen := map[string]bool{}
	for _, t := range tags {
		lt := strings.ToLower(t)
		for _, q := range quantFormats {
			if lt == q {
				seen[q] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for _, q := range quantFormats { // stable order
		if seen[q] {
			out = append(out, q)
		}
	}
	return out
}
