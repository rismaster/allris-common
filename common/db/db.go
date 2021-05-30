package db

import "github.com/rismaster/allris-common/common/slog"

func DoInBatch(batch int, pageSize int, f func(int, int) error) error {
	for i := 0; i < pageSize; i += batch {
		j := i + batch
		if j > pageSize {
			j = pageSize
		}

		slog.Info("do %d:%d", i, j)

		err := f(i, j)
		if err != nil {
			return err
		}
		slog.Info("Done %d/%d", i, j)
	}
	return nil
}
