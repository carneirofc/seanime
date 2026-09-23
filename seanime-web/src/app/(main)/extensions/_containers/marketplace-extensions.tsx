import { Extension_Type } from "@/api/generated/types"
import { useGetAllExtensions, useGetMarketplaceExtensions } from "@/api/hooks/extensions.hooks"
import { MarketplaceExtensionCard } from "@/app/(main)/extensions/_containers/marketplace-extension-card"
import { groupExtensionsByType, matchesExtensionSearch, normalizeSearchTerm } from "@/app/(main)/extensions/_lib/extension-filters"
import { DEFAULT_MARKETPLACE_URL, marketplaceUrlAtom } from "@/app/(main)/extensions/_lib/marketplace.atoms"
import { LANGUAGES_LIST } from "@/app/(main)/manga/_lib/language-map"
import { LuffyError } from "@/components/shared/luffy-error"
import { Alert } from "@/components/ui/alert"
import { AppLayoutStack } from "@/components/ui/app-layout"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Disclosure, DisclosureContent, DisclosureItem, DisclosureTrigger } from "@/components/ui/disclosure"
import { LoadingSpinner } from "@/components/ui/loading-spinner"
import { Modal } from "@/components/ui/modal"
import { Popover } from "@/components/ui/popover"
import { Select } from "@/components/ui/select"
import { StaticTabs } from "@/components/ui/tabs"
import { TextInput } from "@/components/ui/text-input"
import { useSearchParams } from "@/lib/navigation"
import { useAtom } from "jotai/react"
import orderBy from "lodash/orderBy"
import React, { useMemo } from "react"
import { AiOutlineExclamationCircle } from "react-icons/ai"
import { BiSearch } from "react-icons/bi"
import { CgMediaPodcast } from "react-icons/cg"
import { LuBlocks, LuBookOpen, LuChevronDown, LuSettings } from "react-icons/lu"
import { MdDataSaverOn } from "react-icons/md"
import { RiFolderDownloadFill } from "react-icons/ri"
import { toast } from "sonner"

const GRID_CLASS = "grid grid-cols-1 lg:grid-cols-3 2xl:grid-cols-4 gap-4"
// Off-screen cards skip layout and paint; a marketplace can list hundreds
const CARD_CLASS = "[content-visibility:auto] [contain-intrinsic-size:auto_180px]"

const CUSTOM_SOURCE_TITLE = <>
    <MdDataSaverOn /> Custom sources <Popover
        className="text-sm"
        trigger={<AiOutlineExclamationCircle className="text-[1.2rem] transition-opacity opacity-45 hover:opacity-90 cursor-pointer" />}
    >
        Custom sources do not provide any streaming features. Torrent and online streaming providers are needed for this.
    </Popover>
</>

const TYPE_SECTIONS: { type: Extension_Type, title: React.ReactNode, description?: string }[] = [
    { type: "plugin", title: <><LuBlocks /> Plugins</> },
    { type: "anime-torrent-provider", title: <><RiFolderDownloadFill />Anime torrents</> },
    { type: "manga-provider", title: <><LuBookOpen />Manga</> },
    { type: "onlinestream-provider", title: <><CgMediaPodcast /> Online streaming</> },
    { type: "custom-source", title: CUSTOM_SOURCE_TITLE, description: "Custom sources let you browse media beyond what AniList provides." },
]

type MarketplaceExtensionsProps = {
    children?: React.ReactNode
}

export function MarketplaceExtensions(props: MarketplaceExtensionsProps) {
    const {
        children,
        ...rest
    } = props

    const [searchTerm, setSearchTerm] = React.useState("")
    const deferredSearchTerm = React.useDeferredValue(searchTerm)
    const [filterType, setFilterType] = React.useState<string>("all")
    const [filterLanguage, setFilterLanguage] = React.useState<string>("all")
    const [marketplaceUrl, setMarketplaceUrl] = useAtom(marketplaceUrlAtom)
    const [isUrlModalOpen, setIsUrlModalOpen] = React.useState(false)
    const [tempUrl, setTempUrl] = React.useState(marketplaceUrl)
    const [urlError, setUrlError] = React.useState("")
    const [isUpdatingUrl, setIsUpdatingUrl] = React.useState(false)
    const isDefaultMarketplace = marketplaceUrl === DEFAULT_MARKETPLACE_URL

    const { data: marketplaceExtensions, isPending: isLoadingMarketplace, refetch } = useGetMarketplaceExtensions(marketplaceUrl)
    const { data: allExtensions } = useGetAllExtensions(false)

    const searchParams = useSearchParams()
    React.useEffect(() => {
        const type = searchParams.get("type")
        if (type) {
            setFilterType(type)
        }
    }, [searchParams])

    function formatMissingTypes(types: string[]) {
        if (types.length <= 1) return types[0] ?? ""
        if (types.length === 2) return `${types[0]} and ${types[1]}`

        return `${types.slice(0, -1).join(", ")}, and ${types.at(-1)}`
    }

    const installedIds = useMemo(() => new Set([
        ...(allExtensions?.extensions ?? []),
        ...(allExtensions?.disabledExtensions ?? []),
        ...(allExtensions?.invalidExtensions ?? []),
    ].map(ext => ext.id)), [allExtensions])

    // Filter extensions based on search term, filter type, and language
    const filteredExtensions = React.useMemo(() => {
        if (!marketplaceExtensions) return []

        const term = normalizeSearchTerm(deferredSearchTerm)
        const lang = filterLanguage.toLowerCase()
        const filtered = marketplaceExtensions.filter(ext =>
            (filterType === "all" || ext.type === filterType)
            && (filterLanguage === "all" || ext.lang?.toLowerCase() === lang)
            && matchesExtensionSearch(ext, term),
        )
        return orderBy(filtered, ["name", "manifestUri"])
    }, [marketplaceExtensions, deferredSearchTerm, filterType, filterLanguage])

    // Get available languages from extensions
    const availableLanguages = useMemo(() => {
        if (!marketplaceExtensions) return []

        // Get unique languages from extensions
        const langSet = new Set<string>()
        marketplaceExtensions.forEach(ext => {
            if (ext.lang) langSet.add(ext.lang.toLowerCase())
        })

        // Convert to array and sort
        return Array.from(langSet).sort()
    }, [marketplaceExtensions])

    // Create language options for dropdown
    const languageOptions = useMemo(() => {
        const options = [{ value: "all", label: "All Languages" }]

        availableLanguages.forEach(langCode => {
            const langInfo = LANGUAGES_LIST[langCode]
            if (langInfo) {
                options.push({
                    value: langCode,
                    label: langInfo.name || langCode.toUpperCase(),
                })
            } else {
                options.push({
                    value: langCode,
                    label: langCode.toUpperCase(),
                })
            }
        })

        return options
    }, [availableLanguages])

    const missingDefaultTypes = useMemo(() => {
        if (!isDefaultMarketplace || !marketplaceExtensions) return []

        return [
            { type: "onlinestream-provider", label: "online streaming" },
            { type: "anime-torrent-provider", label: "torrent streaming" },
            { type: "manga-provider", label: "manga" },
        ].filter(item => !marketplaceExtensions.some(ext => ext.type === item.type))
            .map(item => item.label)
    }, [isDefaultMarketplace, marketplaceExtensions])

    const groups = groupExtensionsByType(filteredExtensions)

    // validate URL
    const validateUrl = (url: string): boolean => {
        try {
            new URL(url)
            setUrlError("")
            return true
        }
        catch (e) {
            // Bare absolute filesystem paths (no URL scheme) are also valid — the
            // backend accepts these for local/monorepo marketplaces.
            if (/^\//.test(url) || /^[a-zA-Z]:[\\/]/.test(url)) {
                setUrlError("")
                return true
            }
            setUrlError("Please enter a valid URL")
            return false
        }
    }

    // handle URL change
    const handleUrlChange = async () => {
        if (validateUrl(tempUrl)) {
            setIsUpdatingUrl(true)
            try {
                setMarketplaceUrl(tempUrl)
                await refetch()
                setIsUrlModalOpen(false)
                toast.success("Marketplace URL updated")
            }
            catch (error) {
                toast.error("Failed to fetch extensions from the provided URL")
                console.error("Error fetching extensions:", error)
            }
            finally {
                setIsUpdatingUrl(false)
            }
        }
    }

    // apply default URL immediately
    const applyDefaultUrl = async () => {
        setIsUpdatingUrl(true)
        try {
            setMarketplaceUrl(DEFAULT_MARKETPLACE_URL)
            await refetch()
            setIsUrlModalOpen(false)
            toast.success("Reset to default marketplace URL")
        }
        catch (error) {
            toast.error("Failed to fetch extensions from the default URL")
            console.error("Error fetching extensions:", error)
        }
        finally {
            setIsUpdatingUrl(false)
        }
    }

    return (
        <AppLayoutStack className="gap-6">
            <Modal
                open={isUrlModalOpen}
                onOpenChange={setIsUrlModalOpen}
                title="Repository URL"
            >
                <div className="space-y-4">
                    <p className="text-sm text-(--muted)">
                        Enter the URL of the repository JSON file, or an absolute local file path / file:// URL
                        for a marketplace checked out on the server's filesystem.
                    </p>

                    <p className="text-sm text-(--muted)">
                        For private GitHub repositories, embed a personal access token in the URL
                        (e.g. https://&lt;token&gt;@raw.githubusercontent.com/...) or set the SEANIME_GITHUB_TOKEN
                        environment variable on the server.
                    </p>

                    <Disclosure type="single" collapsible>
                        <DisclosureItem value="local-repo-help">
                            <DisclosureTrigger>
                                <Button
                                    intent="gray-outline"
                                    size="sm"
                                    className="w-full justify-between"
                                    rightIcon={<LuChevronDown />}
                                >
                                    Using a local repository?
                                </Button>
                            </DisclosureTrigger>
                            <DisclosureContent className="pt-3">
                                <div className="space-y-3 text-sm text-(--muted)">
                                    <div>
                                        <p className="font-medium text-(--foreground)">Local file path</p>
                                        <p>
                                            The server reads the marketplace file directly off its own filesystem — use an
                                            absolute path, either bare or as a file:// URL. Relative paths are not
                                            accepted here (only inside the marketplace JSON, see below).
                                        </p>
                                        <pre className="mt-2 rounded-[--radius-md] bg-gray-950 p-2 text-xs overflow-x-auto">
{`# Linux/macOS
/home/user/my-extensions/marketplace.json
file:///home/user/my-extensions/marketplace.json

# Windows
file:///C:/Users/me/my-extensions/marketplace.json`}
                                        </pre>
                                    </div>

                                    <div>
                                        <p className="font-medium text-(--foreground)">Monorepo layout</p>
                                        <p>
                                            A local marketplace.json can reference sibling extensions by a path relative
                                            to its own location, and each manifest can in turn reference its payload the
                                            same way — so a whole repository can be checked out and referenced as-is.
                                            An entry only needs an id and a manifestURI; everything shown on its card is
                                            read from the manifest it points at.
                                        </p>
                                        <pre className="mt-2 rounded-[--radius-md] bg-gray-950 p-2 text-xs overflow-x-auto">
{`my-extensions/
├── marketplace.json
└── extensions/
    └── extension-example/
        ├── manifest.json
        └── payload.js`}
                                        </pre>
                                        <pre className="mt-2 rounded-[--radius-md] bg-gray-950 p-2 text-xs overflow-x-auto">
{`// marketplace.json
[
  {
    "id": "extension-example",
    "manifestURI": "extensions/extension-example/manifest.json"
  }
]`}
                                        </pre>
                                        <pre className="mt-2 rounded-[--radius-md] bg-gray-950 p-2 text-xs overflow-x-auto">
{`// extensions/extension-example/manifest.json
{
  "id": "extension-example",
  "payloadURI": "payload.js",
  ...
}`}
                                        </pre>
                                    </div>
                                </div>
                            </DisclosureContent>
                        </DisclosureItem>
                    </Disclosure>

                    <TextInput
                        label="Marketplace URL"
                        value={tempUrl}
                        onValueChange={(value) => {
                            setTempUrl(value)
                            // Validate as user types, but only if there's some input
                            if (value) validateUrl(value)
                        }}
                        error={urlError}
                        placeholder="https://example.com/marketplace.json or file:///path/to/marketplace.json"
                    />

                    <div className="flex justify-between">
                        <div className="flex gap-2">
                            <Button
                                intent="primary-subtle"
                                onClick={applyDefaultUrl}
                                loading={isUpdatingUrl}
                                disabled={isUpdatingUrl}
                            >
                                Apply Default
                            </Button>
                        </div>

                        <div className="flex gap-2">
                            <Button
                                intent="gray-outline"
                                onClick={() => setIsUrlModalOpen(false)}
                            >
                                Cancel
                            </Button>

                            <Button
                                intent="primary"
                                onClick={handleUrlChange}
                                disabled={!tempUrl || !!urlError || isUpdatingUrl}
                                loading={isUpdatingUrl}
                            >
                                Save
                            </Button>
                        </div>
                    </div>
                </div>
            </Modal>

            <div className="flex items-center gap-2 flex-wrap">
                <div>
                    <h2>
                        Marketplace
                    </h2>
                    <p className="text-(--muted) text-sm">
                        Browse and install extensions from the repository.
                    </p>
                    <p className="text-(--muted) text-xs mt-1">
                        Source: {marketplaceUrl === DEFAULT_MARKETPLACE_URL ?
                        <span>Official repository</span> :
                        <span>{marketplaceUrl}</span>
                    }
                    </p>
                </div>

                <div className="flex flex-1"></div>

                <div className="flex items-center gap-2">
                    <Button
                        className="rounded-full"
                        intent="gray-outline"
                        onClick={() => {
                            refetch()
                            toast.success("Refreshed", { duration: 1000 })
                        }}
                    >
                        Refresh
                    </Button>
                    <Button
                        className="rounded-full"
                        intent="gray-outline"
                        leftIcon={<LuSettings />}
                        onClick={() => {
                            setTempUrl(marketplaceUrl)
                            setUrlError("")
                            setIsUrlModalOpen(true)
                        }}
                    >
                        Change repository
                    </Button>
                </div>
            </div>

            <div className="flex flex-wrap gap-4">
                {!!missingDefaultTypes.length && (
                    <Alert
                        intent="warning"
                        title="No content providers available"
                        description={<div>
                            <p>The Seanime default marketplace no longer indexes content providers. Find a new repository URL online and add it.</p>
                            <Button
                                intent="primary"
                                size="sm"
                                className="mt-2"
                                onClick={() => {
                                    setTempUrl("")
                                    setUrlError("")
                                    setIsUrlModalOpen(true)
                                }}
                            >
                                Add new repository
                            </Button>
                        </div>}
                        className="w-full"
                    />
                )}

                <StaticTabs
                    className="w-fit border rounded-full py-0"
                    triggerClass="px-4 py-2 text-sm h-full rounded-full"
                    pillClass="rounded-full border-transparent"
                    items={[
                        {
                            name: "All Types",
                            isCurrent: filterType === "all",
                            onClick: () => setFilterType("all"),
                            // iconType: IoGrid,
                        },
                        {
                            name: "Plugins",
                            isCurrent: filterType === "plugin",
                            onClick: () => setFilterType("plugin"),
                            // iconType: LuBlocks,
                        },
                        {
                            name: "Anime Torrents",
                            isCurrent: filterType === "anime-torrent-provider",
                            onClick: () => setFilterType("anime-torrent-provider"),
                            // iconType: RiFolderDownloadFill,
                        },
                        {
                            name: "Manga",
                            isCurrent: filterType === "manga-provider",
                            onClick: () => setFilterType("manga-provider"),
                            // iconType: LuBookOpen,
                        },
                        {
                            name: "Online Streaming",
                            isCurrent: filterType === "onlinestream-provider",
                            onClick: () => setFilterType("onlinestream-provider"),
                            // iconType: CgMediaPodcast,
                        },
                        {
                            name: "Custom Sources",
                            isCurrent: filterType === "custom-source",
                            onClick: () => setFilterType("custom-source"),
                            // iconType: CgMediaPodcast,
                        },
                    ]}
                />

                <div className="flex flex-col lg:flex-row w-full gap-2">
                    <Select
                        value={filterLanguage}
                        onValueChange={setFilterLanguage}
                        options={languageOptions}
                        fieldClass="lg:max-w-[200px]"
                    />
                    <TextInput
                        placeholder="Search extensions..."
                        value={searchTerm}
                        onValueChange={(v) => setSearchTerm(v)}
                        className="pl-10"
                        leftIcon={<BiSearch />}
                    />
                </div>
            </div>

            {isLoadingMarketplace && <LoadingSpinner />}

            {(!marketplaceExtensions && !isLoadingMarketplace) && <LuffyError>
                Could not get marketplace extensions.
            </LuffyError>}

            {(!!marketplaceExtensions && filteredExtensions.length === 0) && (
                <Card className="p-8 text-center">
                    <p className="text-(--muted)">No extensions found matching your criteria.</p>
                </Card>
            )}

            {TYPE_SECTIONS.map(section => !!groups[section.type].length && (
                <Card key={section.type} className="p-4 space-y-6">
                    <div>
                        <h3 className="flex gap-3 items-center">{section.title}</h3>
                        {section.description && <p className="text-(--muted) text-sm">{section.description}</p>}
                    </div>
                    <div className={GRID_CLASS}>
                        {groups[section.type].map(extension => (
                            <MarketplaceExtensionCard
                                key={extension.id}
                                extension={extension}
                                isInstalled={installedIds.has(extension.id)}
                                className={CARD_CLASS}
                            />
                        ))}
                    </div>
                </Card>
            ))}
            {/* Anything whose type matches none of the groups above would otherwise be counted as a result and rendered nowhere */}
            {!!groups.other.length && (
                <Card className="p-4 space-y-6">
                    <h3 className="flex gap-3 items-center"><LuBlocks /> Other</h3>
                    <div className={GRID_CLASS}>
                        {groups.other.map(extension => (
                            <MarketplaceExtensionCard
                                key={extension.id}
                                extension={extension}
                                isInstalled={installedIds.has(extension.id)}
                                className={CARD_CLASS}
                                showType
                            />
                        ))}
                    </div>
                </Card>
            )}
        </AppLayoutStack>
    )
}
