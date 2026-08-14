package plugin_ui

import (
	"seanime/internal/goja/goja_bindings"

	"github.com/dop251/goja"
)

// bindFetch binds $fetch. It deliberately has no access to the AniList token: a
// plugin that needs one asks for the anilist-token permission and reads it through
// $database, rather than having it handed to the binding that talks to arbitrary
// hosts.
func (c *Context) bindFetch(obj *goja.Object, allowedDomains []string) {
	f := goja_bindings.NewFetch(c.ext.ID, c.vm, allowedDomains)

	_ = obj.Set("fetch", f.Fetch)

	go func() {
		for fn := range f.ResponseChannel() {
			c.scheduler.ScheduleAsync(func() error {
				fn()
				return nil
			})
		}
	}()

	c.registerOnCleanup(func() {
		c.logger.Debug().Msg("plugin: Terminating fetch")
		f.Close()
	})
}

func (c *Context) bindAbortContext() {
	goja_bindings.BindAbortContext(c.vm, c.scheduler)
}
