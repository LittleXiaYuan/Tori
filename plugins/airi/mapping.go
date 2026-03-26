package airi

// EmotionAct represents the ACT tag emotion payload.
type EmotionAct struct {
	Emotion struct {
		Name      string `json:"name"`
		Intensity int    `json:"intensity"`
	} `json:"emotion"`
}

// mapEmotionToVRM converts Yunque's Plutchik-like emotion string to VRM/Live2D expressions.
// Airi valid emotions: happy, sad, angry, think, surprised, awkward, question, curious, neutral
func mapEmotionToVRM(yunqueEmotion string) string {
	switch yunqueEmotion {
	case "joy", "happy", "amusement", "excitement", "love":
		return "happy"
	case "anger", "angry", "annoyance", "disapproval", "disgust", "loathing":
		return "angry"
	case "sadness", "sad", "grief", "remorse", "boredom":
		return "sad"
	case "fear", "terror", "apprehension", "surprise", "amazement", "distraction":
		return "surprised"
	case "trust", "acceptance", "admiration":
		return "happy"
	case "curiosity", "interest", "anticipation":
		return "curious"
	case "awkward", "embarrassment", "shame":
		return "awkward"
	case "question", "confusion", "puzzlement":
		return "question"
	case "think", "contemplation", "thoughtful":
		return "think"
	default:
		return "neutral"
	}
}
