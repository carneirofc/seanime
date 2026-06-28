//go:build windows

package handlers

import "golang.org/x/sys/windows"

func getPathDiskUsage(path string) (DiskUsageInfo, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return DiskUsageInfo{}, err
	}

	var freeBytes, totalBytes, totalFree uint64
	err = windows.GetDiskFreeSpaceEx(p, &freeBytes, &totalBytes, &totalFree)
	if err != nil {
		return DiskUsageInfo{}, err
	}

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
