import { defineConfig, loadEnv, RsbuildPluginAPI } from "@rsbuild/core"
import { pluginBabel } from "@rsbuild/plugin-babel"
import { pluginNodePolyfill } from "@rsbuild/plugin-node-polyfill"
import { pluginReact } from "@rsbuild/plugin-react"
import { RsdoctorRspackPlugin } from "@rsdoctor/rspack-plugin"
import { TanStackRouterRspack } from "@tanstack/router-plugin/rspack"
import { buildSync } from "esbuild"
import * as fs from "node:fs"
import path from "path"

const { publicVars } = loadEnv({ prefixes: ["SEA_"] })

const isElectronDesktop = process.env.SEA_PUBLIC_DESKTOP === "electron"
const distPath = isElectronDesktop ? "out-denshi" : "out"

/**
 * Resolves an installed package's directory, tolerating either npm layout:
 * nested in seanime-web/node_modules, or hoisted to the workspace root.
 * Falls back to the local path so the alias still points somewhere sane.
 */
function resolvePackageDir(name: string): string {
    const candidates = [
        path.resolve(__dirname, "node_modules", name),
        path.resolve(__dirname, "..", "node_modules", name),
    ]
    return candidates.find(candidate => fs.existsSync(candidate)) ?? candidates[0]
}

export default defineConfig({
    plugins: [
        pluginReact(),
        pluginNodePolyfill({
            include: ["buffer", "crypto"],
        }),
        { // run stuff before build
            name: "before-build",
            setup(api: RsbuildPluginAPI) {
                // api.onBeforeStartDevServer(processJassub)
                api.onBeforeBuild(processJassub)

                function processJassub() {
                    console.log("Running transpilation...")
                    const jassubRoot = path.dirname(require.resolve("jassub/package.json"))
                    const source = path.join(jassubRoot, "dist/worker/worker.js")
                    const outDir = path.resolve(__dirname, "public", "jassub")
                    const outFile = path.join(outDir, "jassub-worker.js")

                    if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true })

                    // transpile using esbuild (goated)
                    buildSync({
                        entryPoints: [source],
                        outfile: outFile,
                        bundle: true,
                        format: "iife",
                        define: {
                            "import.meta.url": "self.location.href",
                        },
                        minify: false,
                    })

                    // copy wasm files
                    const wasmSource = path.join(jassubRoot, "dist/wasm/jassub-worker.wasm")
                    const wasmModernSource = path.join(jassubRoot, "dist/wasm/jassub-worker-modern.wasm")
                    fs.copyFileSync(wasmSource, path.join(outDir, "jassub-worker.wasm"))
                    fs.copyFileSync(wasmModernSource, path.join(outDir, "jassub-worker-modern.wasm"))
                    console.log("Finished transpiling")
                }
            },
        },
        pluginBabel({
            include: /\.(?:jsx|tsx)$/,
            babelLoaderOptions(opts) {
                opts.plugins ??= []
                opts.plugins.push(["babel-plugin-react-compiler", { runtimeModule: "react-compiler-runtime" }])
            },
        }),
    ].filter(Boolean),
    source: {
        entry: {
            index: "./src/main.tsx",
        },
        define: {
            ...publicVars,
            "process.env.NEXT_PUBLIC_WS_DEBUG": JSON.stringify(process.env.NEXT_PUBLIC_WS_DEBUG ?? ""),
            "process.env.NEXT_PUBLIC_DEBUG": JSON.stringify(process.env.NEXT_PUBLIC_DEBUG ?? ""),
            "process.env.NEXT_PUBLIC_WS_RTT_WARN_MS": JSON.stringify(process.env.NEXT_PUBLIC_WS_RTT_WARN_MS ?? ""),
            "process.env.NEXT_PUBLIC_API_DEBUG": JSON.stringify(process.env.NEXT_PUBLIC_API_DEBUG ?? ""),
            "process.env.NEXT_PUBLIC_API_SLOW_MS": JSON.stringify(process.env.NEXT_PUBLIC_API_SLOW_MS ?? ""),
        },
    },
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
            // Pin a single copy of React. npm workspaces hoist dependencies to the repo root,
            // so resolve against whichever node_modules actually holds the package instead of
            // assuming it is nested under seanime-web/.
            "react": resolvePackageDir("react"),
            "react-dom": resolvePackageDir("react-dom"),
            // jassub ships a default subtitle font we replace with Roboto.
            [path.join(resolvePackageDir("jassub"), "dist/default.woff2")]: path.resolve(__dirname, "public/fonts/Roboto-Medium.ttf"),
        },
    },
    server: { // dev server
        port: 43210,
        host: "0.0.0.0",
        headers: {
            "Cross-Origin-Embedder-Policy": "credentialless",
            "Cross-Origin-Opener-Policy": "same-origin",
        },
    },
    output: {
        cleanDistPath: true,
        sourceMap: !!process.env.RSDOCTOR,
        distPath: {
            root: distPath,
        },
        filename: {
            js: process.env.NODE_ENV === "production" ? "[name].[contenthash:8].js" : "[name].js",
            css: process.env.NODE_ENV === "production" ? "[name].[contenthash:8].css" : "[name].css",
        },
    },
    html: {
        template: "./index.html",
        title: "Seanime",
    },
    performance: {
        chunkSplit: {
            forceSplitting: {
                "hls": /hls\.js/,
                "recorder": /rrweb/,
            },
        },
    },
    tools: {
        // swc: {
        //   minify: true,
        // },
        rspack: {
            experiments: {
                // breaks rrweb
                // outputModule: true,
            },
            output: { // redundant?
                chunkFilename: process.env.NODE_ENV === "production" ? "static/js/async/[name].[contenthash:8].js" : "static/js/async/[name].js",
            },
            optimization: {
                chunkIds: !!process.env.RSDOCTOR ? "named" : undefined,
            },
            plugins: [
                TanStackRouterRspack({
                    routesDirectory: "./src/routes",
                    generatedRouteTree: "./src/routeTree.gen.ts",
                    autoCodeSplitting: true,
                }),
                process.env.RSDOCTOR && new RsdoctorRspackPlugin({}),
            ].filter(Boolean),
            resolve: {
                fallback: {
                    module: false,
                },
            },
            module: {
                noParse: /[\\/]jassub[\\/]dist[\\/]wasm[\\/]jassub-worker\.js$/,
                rules: [
                    { // stops circular deps warning (worker + pthread detection)
                        test: /\.js$/,
                        include: /[\\/]jassub[\\/]dist[\\/]/,
                        parser: {
                            worker: false,
                            url: false,
                        },
                    },
                    { // don't emit these again
                        test: /\.wasm$/,
                        include: /node_modules[\\/]jassub/,
                        type: "asset/resource",
                        generator: {
                            emit: false,
                        },
                    },
                ],
            },
        },
    },
})
