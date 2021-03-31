package util

func RetryWhenFailed(retryCount int, f func() error) {
	for i := 0; i < retryCount+1; i++ {
		err := f()
		if err == nil {
			return
		}
	}
	// todo log
}
