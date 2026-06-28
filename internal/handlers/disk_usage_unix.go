//go:build !windows

package handlers

import "golang.org/x/sys/unix"

func getPathDiskUsage(path string) (DiskUsageInfo, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return DiskUsageInfo{}, err
	}

	bsize := uint64(stat.Bsize)
	totalBytes := stat.Blocks * bsize
	freeBytes := stat.Bavail * bsize

	const gb = float64(1 << 30)
	totalGB := float64(totalBytes) / gb
	freeGB := float64(freeBytes) / gb
	usedGB := totalGB - freeGB

	usedPct, freePct := 0.0, 0.0
	if totalBytes > 0 {
		usedPct = float64(totalBytes-freeBytes) / float64(totalBytes) * 100
		freePct = float64(freeBytes) / float64(totalBytes) * 100
	}

	return DiskUsageInfo{
		Path:    path,
		TotalGB: totalGB,
		UsedGB:  usedGB,
		FreeGB:  freeGB,
		UsedPct: usedPct,
		FreePct: freePct,
	}, nil
}
