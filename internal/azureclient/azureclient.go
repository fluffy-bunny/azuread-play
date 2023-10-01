package azureclient

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	graph "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphgocore "github.com/microsoftgraph/msgraph-sdk-go-core"
	models "github.com/microsoftgraph/msgraph-sdk-go/models"
	microsoftgraph_users "github.com/microsoftgraph/msgraph-sdk-go/users"
)

type User struct {
	ID          string
	DisplayName string
	Mail        string
	Groups      []string
}
type AAD struct {
	graphClient *graph.GraphServiceClient
}

func NewClient(tenantID string, clientID string, clientSecret string) (aad *AAD, err error) {
	aad = &AAD{}
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, err
	}
	aad.graphClient, err = graph.NewGraphServiceClientWithCredentials(
		cred, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		return nil, err
	}
	return aad, nil
}

func (aad *AAD) IterateUsers(ctx context.Context, callback func(user *User) bool) error {
	headers := abstractions.NewRequestHeaders()
	headers.Add("ConsistencyLevel", "eventual")

	requestCount := true

	options := &microsoftgraph_users.UsersRequestBuilderGetRequestConfiguration{
		Headers: headers,
		QueryParameters: &microsoftgraph_users.UsersRequestBuilderGetQueryParameters{
			Count: &requestCount,
			Select: []string{
				"id",
				"displayName",
				"mail",
			},
		},
	}
	result, err := aad.graphClient.Users().
		Get(ctx, options)
	if err != nil {
		return err
	}

	pageIterator, err := msgraphgocore.NewPageIterator[models.Userable](result, aad.graphClient.GetAdapter(), models.CreateUserCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return err
	}
	err = pageIterator.Iterate(ctx, func(user models.Userable) bool {
		u := User{
			ID:          *user.GetId(),
			DisplayName: *user.GetDisplayName(),
			Mail:        *user.GetMail(),
		}
		canContinue := callback(&u)
		return canContinue
	})
	if err != nil {
		return err
	}

	return nil
}

func (aad *AAD) IterateUserGroups(ctx context.Context, userID string, callback func(group string) bool) error {
	result, err := aad.graphClient.Users().
		ByUserId(userID).
		MemberOf().
		Get(ctx, nil)
	if err != nil {
		return err
	}

	pageIterator, err := msgraphgocore.NewPageIterator[models.DirectoryObjectable](result, aad.graphClient.GetAdapter(), models.CreateDirectoryObjectCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return err
	}
	err = pageIterator.Iterate(ctx, func(do models.DirectoryObjectable) bool {
		group, ok := do.(*models.Group)
		if !ok {
			return true
		}
		displayName := group.GetDisplayName()
		canContinue := callback(*displayName)
		return canContinue

	})

	return nil
}
