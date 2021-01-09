package common

func AppendPrefix(prefix byte, data []byte) []byte {
	res := make([]byte, len(data)+1)
	res[0] = prefix
	for i := 1; i < len(data)+1; i++ {
		res[i] = data[i-1]
	}
	return res
}
