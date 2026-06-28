import { HIDE_IMAGES } from "@/types/constants"
import { IconButton, IconButtonProps } from "@/components/ui/button"
import { cn } from "@/components/ui/core/styling"
import { getImageProxyFallbackUrl, isImageProxyUrl } from "@/lib/server/assets"
import React, { forwardRef, useEffect, useRef, useState } from "react"
import { LuRefreshCw } from "react-icons/lu"

const DEFAULT_FALLBACK_IMAGE = "/no-cover.png"
const IMAGE_PROXY_RETRY_DELAY_MS = 1000
const IMAGE_PROXY_MAX_RETRIES = 2

function withImageRetryNonce(src: string, nonce: number) {
    if (!nonce) {
        return src
    }

    try {
        const baseUrl = typeof window !== "undefined" ? window.location.origin : "http://localhost"
        const parsed = new URL(src, baseUrl)
        parsed.searchParams.set("__sea_image_retry", `${nonce}`)

        if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(src)) {
            return parsed.toString()
        }

        return `${parsed.pathname}${parsed.search}${parsed.hash}`
    } catch {
        const separator = src.includes("?") ? "&" : "?"
        return `${src}${separator}__sea_image_retry=${nonce}`
    }
}

type ImageProps = React.ImgHTMLAttributes<HTMLImageElement> & {
    fill?: boolean
    priority?: boolean
    overrideSrc?: string
    quality?: number | string
    placeholder?: string
    blurDataURL?: string
    sizes?: string
    allowGif?: boolean
}

export type SeaImageRetryButtonProps = Omit<IconButtonProps, "icon">

export const SeaImageRetryButton = forwardRef<HTMLButtonElement, SeaImageRetryButtonProps>(
    ({ className, size = "sm", intent = "white", ...props }, ref) => (
        <IconButton
            ref={ref}
            size={size}
            intent={intent}
            rounded
            aria-label={props["aria-label"] ?? "Retry image"}
            title={props.title ?? "Retry image"}
            className={cn(
                "border border-white/20 bg-black/50 text-white backdrop-blur-sm hover:bg-white hover:text-black",
                className,
            )}
            icon={<LuRefreshCw className="size-4" aria-hidden="true" />}
            data-proxied-image-retry-button
            {...props}
        />
    ),
)

SeaImageRetryButton.displayName = "SeaImageRetryButton"

export const SeaImage = forwardRef<HTMLImageElement, ImageProps & { isExternal?: boolean }>(
    ({ isExternal, fill, priority, quality, placeholder, sizes, allowGif, overrideSrc, onError, onLoad, ...props }, ref) => {
        const [hasError, setHasError] = useState(false)
        const [proxyRetryCount, setProxyRetryCount] = useState(0)
        const [proxyRetryNonce, setProxyRetryNonce] = useState(0)
        const [usingDirectFallback, setUsingDirectFallback] = useState(false)
        const retryTimerRef = useRef<number | null>(null)

        const imageSrc = props.src
        const isStringSrc = typeof imageSrc === "string"
        const isProxiedImage = isStringSrc && isImageProxyUrl(imageSrc)
        const directFallbackSrc = isProxiedImage ? getImageProxyFallbackUrl(imageSrc) : undefined

        useEffect(() => {
            setHasError(false)
            setProxyRetryCount(0)
            setProxyRetryNonce(0)
            setUsingDirectFallback(false)

            return () => {
                if (retryTimerRef.current) {
                    clearTimeout(retryTimerRef.current)
                    retryTimerRef.current = null
                }
            }
        }, [imageSrc])

        if (HIDE_IMAGES) {
            return <Image
                ref={ref}
                {...props}
                src={DEFAULT_FALLBACK_IMAGE}
                className={props.className}
                alt={props.alt || "cover"}
                fill={fill}
            />
        }

        const blocked = isExternal && props.src && typeof props.src === "string" && !(
            props.src.endsWith(".png")
            || props.src.endsWith(".jpg")
            || props.src.endsWith(".jpeg")
            || props.src.endsWith(".avif")
            || props.src.endsWith(".webp")
            || props.src.endsWith(".ico")
            || (allowGif && props.src.endsWith(".gif"))
        )

        const fallbackImageSrc = blocked ? DEFAULT_FALLBACK_IMAGE : (overrideSrc || DEFAULT_FALLBACK_IMAGE)

        const clearRetryTimer = () => {
            if (retryTimerRef.current) {
                clearTimeout(retryTimerRef.current)
                retryTimerRef.current = null
            }
        }

        const retryImage = () => {
            clearRetryTimer()
            setHasError(false)
            setProxyRetryCount(0)
            setUsingDirectFallback(false)
            setProxyRetryNonce(Date.now())
        }

        const queueRetry = () => {
            clearRetryTimer()
            retryTimerRef.current = window.setTimeout(() => {
                setProxyRetryNonce(Date.now())
                retryTimerRef.current = null
            }, IMAGE_PROXY_RETRY_DELAY_MS)
        }

        const currentSrc = usingDirectFallback && directFallbackSrc
            ? directFallbackSrc
            : isStringSrc && isProxiedImage
                ? withImageRetryNonce(imageSrc, proxyRetryNonce)
                : imageSrc || ""

        function handleError(event: React.SyntheticEvent<HTMLImageElement, Event>) {
            onError?.(event)

            if (blocked) {
                setHasError(true)
                return
            }

            if (isProxiedImage && !usingDirectFallback && proxyRetryCount < IMAGE_PROXY_MAX_RETRIES) {
                setProxyRetryCount(prev => prev + 1)
                queueRetry()
                return
            }

            if (isProxiedImage && !usingDirectFallback && directFallbackSrc) {
                clearRetryTimer()
                setUsingDirectFallback(true)
                return
            }

            setHasError(true)
            console.warn(`Error loading image ${imageSrc}`)
        }

        function handleLoad(event: React.SyntheticEvent<HTMLImageElement, Event>) {
            clearRetryTimer()
            onLoad?.(event)
        }

        if (isProxiedImage && hasError) {
            return (
                <div
                    className={cn(
                        "relative overflow-hidden",
                        fill && "absolute inset-0",
                    )}
                >
                    <Image
                        ref={ref}
                        {...props}
                        src={fallbackImageSrc}
                        alt={props.alt || ""}
                        fill={fill}
                        priority={priority}
                        placeholder={placeholder}
                    />
                    <div className="pointer-events-none absolute inset-0 z-[1] flex items-center justify-center">
                        <SeaImageRetryButton
                            className="pointer-events-auto"
                            onClick={retryImage}
                        />
                    </div>
                </div>
            )
        }

        return <Image
            ref={ref}
            {...props}
            src={(hasError ? fallbackImageSrc : currentSrc) || ""}
            alt={props.alt || ""}
            fill={fill}
            priority={priority}
            placeholder={placeholder}
            overrideSrc={hasError ? fallbackImageSrc : overrideSrc}
            onError={handleError}
            onLoad={handleLoad}
        />
    },
)

SeaImage.displayName = "SeaImage"

interface _ImageProps extends React.ImgHTMLAttributes<HTMLImageElement> {
    src: string | any
    alt: string
    width?: number | string
    height?: number | string
    fill?: boolean
    quality?: number | string
    priority?: boolean
    loader?: any
    placeholder?: string
    blurDataURL?: string
    unoptimized?: boolean
    onLoadingComplete?: (img: HTMLImageElement) => void
    layout?: string
    objectFit?: string
    overrideSrc?: string
}

const Image = forwardRef<HTMLImageElement, _ImageProps>((
    {
        src,
        alt,
        width,
        height,
        fill,
        style,
        className,
        quality,
        priority,
        loader,
        placeholder,
        blurDataURL,
        unoptimized,
        onLoadingComplete,
        layout,
        objectFit,
        overrideSrc,
        onLoad,
        ...props
    },
    ref,
) => {
    const [isLoaded, setIsLoaded] = useState(false)

    const isStaticImport = typeof src === "object" && src !== null && "src" in src
    const imageSrc = overrideSrc || (isStaticImport ? src.src : src)

    const staticBlur = isStaticImport ? src.blurDataURL : undefined

    useEffect(() => {
        setIsLoaded(false)
    }, [imageSrc])

    const blurUrl = (placeholder && placeholder !== "blur" && placeholder !== "empty")
        ? placeholder
        : (placeholder === "blur" ? (blurDataURL || staticBlur) : undefined)

    const fillStyle: React.CSSProperties = fill ? {
        position: "absolute",
        height: "100%",
        width: "100%",
        left: 0,
        top: 0,
        right: 0,
        bottom: 0,
        color: "transparent",
    } : {}

    const placeholderStyle: React.CSSProperties = (blurUrl && !isLoaded) ? {
        backgroundImage: `url("${blurUrl}")`,
        backgroundSize: objectFit === "contain" ? "contain" : "cover",
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
    } : {}

    const imageWidth = fill ? undefined : (width || (isStaticImport ? src.width : undefined))
    const imageHeight = fill ? undefined : (height || (isStaticImport ? src.height : undefined))

    return (
        <img
            ref={ref}
            src={imageSrc}
            alt={alt}
            width={imageWidth}
            height={imageHeight}
            decoding="async"
            loading={priority ? "eager" : "lazy"}
            className={className}
            style={{
                ...fillStyle,
                ...placeholderStyle,
                ...(objectFit ? { objectFit: objectFit as any } : {}),
                ...style,
            }}
            onLoad={(e) => {
                setIsLoaded(true)
                onLoad?.(e)
            }}
            {...props}
        />
    )
})

Image.displayName = "Image"
