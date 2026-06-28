import { useServerQuery } from "@/api/client/requests"

export type DiskUsageInfo = {
    path: string
    totalGB: number
    usedGB: number
    freeGB: number
    usedPct: number
    freePct: number
}

export type LibraryDiskUsageResponse = {
    paths: DiskUsageInfo[]
}

export function useGetLibraryDiskUsage() {
    return useServerQuery<LibraryDiskUsageResponse>({
        endpoint: "/api/v1/library/disk-usage",
        method: "GET",
        queryKey: ["library-disk-usage"],
        enabled: true,
    })
}
