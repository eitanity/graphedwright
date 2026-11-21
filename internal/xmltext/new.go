package xmltext

// New makes a writer with its buffer, the one allocation it makes.
func New() *Writer { return &Writer{buf: make([]byte, BufSize)} }
