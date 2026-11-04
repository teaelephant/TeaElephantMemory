package graphql

import (
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
)

func TestExistingCatalogueAdministrationRejectsUsersWithoutReportingLogout(t *testing.T) {
	t.Parallel()

	store := &imageDeletionCatalogue{}
	auth, userToken, _ := imageTestAuth(t, store)
	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)}))
	server.Use(auth.Middleware())

	for _, operation := range []string{
		`deleteTea(id:"11111111-1111-1111-1111-111111111111")`,
		`createTagCategory(name:"Aroma") { id }`,
		`updateTagCategory(id:"11111111-1111-1111-1111-111111111111",name:"Aroma") { id }`,
		`deleteTagCategory(id:"11111111-1111-1111-1111-111111111111")`,
		`createTag(name:"Floral",color:"green",category:"11111111-1111-1111-1111-111111111111") { id }`,
		`updateTag(id:"11111111-1111-1111-1111-111111111111",name:"Floral",color:"green") { id }`,
		`changeTagCategory(id:"11111111-1111-1111-1111-111111111111",category:"22222222-2222-2222-2222-222222222222") { id }`,
		`deleteTag(id:"11111111-1111-1111-1111-111111111111")`,
	} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			for token, code := range map[string]string{"": testImageUnauthorizedCode, userToken: testImageForbiddenCode} {
				reply := imageGraphQLCall(t, server, token, "mutation { "+operation+" }", nil)
				assertRoleDenied(t, reply, code)
			}
		})
	}
}
