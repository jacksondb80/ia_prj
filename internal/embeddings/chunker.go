package embeddings

func Chunk(text string, size int) []string {
	var chunks []string
	for len(text) > size {
		chunks = append(chunks, text[:size])
		text = text[size:]
	}
	if len(text) > 0 {
		chunks = append(chunks, text)
	}
	return chunks
}
