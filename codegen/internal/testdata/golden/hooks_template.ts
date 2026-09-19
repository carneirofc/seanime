//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// anilist
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// export function useGetAnimeCollection() {
//     return useServerQuery<AL_AnimeCollection>({
//         endpoint: API_ENDPOINTS.ANILIST.GetAnimeCollection.endpoint,
//         method: API_ENDPOINTS.ANILIST.GetAnimeCollection.methods[0],
//         queryKey: [API_ENDPOINTS.ANILIST.GetAnimeCollection.key],
//         enabled: true,
//     })
// }

// export function useGetThingById(id: number) {
//     return useServerQuery<AL_Thing>({
//         endpoint: API_ENDPOINTS.ANILIST.GetThingById.endpoint.replace("{id}", String(id)),
//         method: API_ENDPOINTS.ANILIST.GetThingById.methods[0],
//         queryKey: [API_ENDPOINTS.ANILIST.GetThingById.key],
//         enabled: true,
//     })
// }

// export function useSaveThing() {
//     return useServerMutation<AL_Thing, SaveThing_Variables>({
//         endpoint: API_ENDPOINTS.ANILIST.SaveThing.endpoint,
//         method: API_ENDPOINTS.ANILIST.SaveThing.methods[0],
//         mutationKey: [API_ENDPOINTS.ANILIST.SaveThing.key],
//         onSuccess: async () => {
//
//         },
//     })
// }

// export function useMultiMethod() {
//     return useServerQuery({
//         endpoint: API_ENDPOINTS.ANILIST.MultiMethod.endpoint,
//         method: API_ENDPOINTS.ANILIST.MultiMethod.methods[0],
//         queryKey: [API_ENDPOINTS.ANILIST.MultiMethod.key],
//         enabled: true,
//     })
// }

// export function useMultiMethod() {
//     return useServerMutation({
//         endpoint: API_ENDPOINTS.ANILIST.MultiMethod.endpoint,
//         method: API_ENDPOINTS.ANILIST.MultiMethod.methods[1],
//         mutationKey: [API_ENDPOINTS.ANILIST.MultiMethod.key],
//         onSuccess: async () => {
//
//         },
//     })
// }

