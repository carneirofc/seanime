import { defineConfig } from "vitest/config"

/**
 * Rsbuild owns the aliases for the app build; Vitest does not see that config, so the `@/`
 * alias from tsconfig.json has to be repeated here. Without it, any test that pulls in a
 * module with a runtime (non-type-only) `@/` import fails to resolve.
 */
export default defineConfig({
    resolve: {
        alias: {
            "@": new URL("./src", import.meta.url).pathname,
        },
    },
    test: {
        include: ["src/**/*.{test,spec}.{ts,tsx}"],
    },
})
