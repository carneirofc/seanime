package fixtures

// HandleGetThing returns a thing.
//
//	@summary gets a thing.
//	@desc This is the first description.
//	@desc And a second one.
//	@route /api/v1/thing/{id} [GET]
//	@param id - int - true - "The thing id."
//	@returns fixtures.User
func HandleGetThing(c *RouteCtx) error {
	return nil
}

// HandleSaveThing saves a thing.
//
//	@summary saves a thing.
//	@route /api/v1/thing [POST,PATCH]
//	@returns bool
func HandleSaveThing(c *RouteCtx) error {
	type body struct {
		// The thing name.
		Name     string    `json:"name"`
		Optional *int      `json:"optional,omitempty"`
		Media    *AnyThing `json:"media"`
		Inline   struct {
			Enabled bool `json:"enabled"`
		} `json:"inline"`
		InlineSlice []struct {
			Value string `json:"value"`
		} `json:"inlineSlice"`
		InlineMap map[string]struct {
			Value string `json:"value"`
		} `json:"inlineMap"`
	}
	return nil
}

// HandleNoTags has a doc comment but no recognized tags.
func HandleNoTags(c *RouteCtx) error {
	return nil
}

// HandleGetAlpha returns an alpha.
//
//	@summary gets an alpha.
//	@route /api/v1/alpha [GET]
//	@returns alphapkg.Alpha
func HandleGetAlpha(c *RouteCtx) error {
	return nil
}

// HandleGetZeta returns a zeta.
//
//	@summary gets a zeta.
//	@route /api/v1/zeta [GET]
//	@returns zetapkg.Zeta
func HandleGetZeta(c *RouteCtx) error {
	return nil
}

// HandleListUsers returns users, sharing the User type with HandleGetThing so the
// shared-vs-other struct split is exercised too.
//
//	@summary lists users.
//	@route /api/v1/users [GET]
//	@returns []fixtures.User
func HandleListUsers(c *RouteCtx) error {
	return nil
}
