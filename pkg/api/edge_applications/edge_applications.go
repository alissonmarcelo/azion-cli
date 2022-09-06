package edgeapplications

import (
	"context"
	"net/http"
	"time"

	"github.com/aziontech/azion-cli/pkg/cmd/version"
	"github.com/aziontech/azion-cli/utils"
	sdk "github.com/aziontech/azionapi-go-sdk/edgeapplications"
)

type Client struct {
	apiClient *sdk.APIClient
}

type CreateRequest struct {
	sdk.CreateApplicationRequest
}

type UpdateRequest struct {
	sdk.ApplicationUpdateRequest
	Id string
}

type UpdateInstanceRequest struct {
	sdk.ApplicationUpdateInstanceRequest
	Id         string
	IdInstace  string
	FunctionId int64
}

type CreateInstanceRequest struct {
	sdk.ApplicationCreateInstanceRequest
	ApplicationId int64
}

type EdgeApplicationsResponse interface {
	GetId() int64
	GetName() string
}

type UpdateRulesEngineRequest struct {
	sdk.PatchRulesEngineRequest
	IdApplication int64
}

func NewClient(c *http.Client, url string, token string) *Client {
	conf := sdk.NewConfiguration()
	conf.HTTPClient = c
	conf.AddDefaultHeader("Authorization", "token "+token)
	conf.AddDefaultHeader("Accept", "application/json;version=3")
	conf.UserAgent = "Azion_CLI/" + version.BinVersion
	conf.Servers = sdk.ServerConfigurations{
		{URL: url},
	}
	conf.HTTPClient.Timeout = 30 * time.Second

	return &Client{
		apiClient: sdk.NewAPIClient(conf),
	}
}

func (c *Client) Create(ctx context.Context, req *CreateRequest) (EdgeApplicationsResponse, error) {

	request := c.apiClient.EdgeApplicationsMainSettingsApi.EdgeApplicationsPost(ctx).CreateApplicationRequest(req.CreateApplicationRequest)

	edgeApplicationsResponse, httpResp, err := request.Execute()
	if err != nil {
		return nil, utils.ErrorPerStatusCode(httpResp, err)
	}

	return &edgeApplicationsResponse.Results, nil
}

func (c *Client) Update(ctx context.Context, req *UpdateRequest) (EdgeApplicationsResponse, error) {
	request := c.apiClient.EdgeApplicationsMainSettingsApi.EdgeApplicationsIdPatch(ctx, req.Id).ApplicationUpdateRequest(req.ApplicationUpdateRequest)

	edgeApplicationsResponse, httpResp, err := request.Execute()
	if err != nil {
		return nil, utils.ErrorPerStatusCode(httpResp, err)
	}

	return &edgeApplicationsResponse.Results, nil
}

func (c *Client) UpdateInstance(ctx context.Context, req *UpdateInstanceRequest) (EdgeApplicationsResponse, error) {
	request := c.apiClient.EdgeApplicationsEdgeFunctionsInstancesApi.EdgeApplicationsEdgeApplicationIdFunctionsInstancesFunctionsInstancesIdPatch(ctx, req.Id, req.IdInstace).ApplicationUpdateInstanceRequest(req.ApplicationUpdateInstanceRequest)

	req.ApplicationUpdateInstanceRequest.SetName("justfortests2")
	req.SetEdgeFunctionId(req.FunctionId)

	edgeApplicationsResponse, httpResp, err := request.Execute()
	if err != nil {
		return nil, utils.ErrorPerStatusCode(httpResp, err)
	}

	return edgeApplicationsResponse.Results, nil
}

func (c *Client) CreateInstance(ctx context.Context, req *CreateInstanceRequest) (EdgeApplicationsResponse, error) {

	args := make(map[string]interface{})
	req.SetArgs(args)

	request := c.apiClient.EdgeApplicationsEdgeFunctionsInstancesApi.EdgeApplicationsEdgeApplicationIdFunctionsInstancesPost(ctx, req.ApplicationId).ApplicationCreateInstanceRequest(req.ApplicationCreateInstanceRequest)

	edgeApplicationsResponse, httpResp, err := request.Execute()
	if err != nil {
		return nil, utils.ErrorPerStatusCode(httpResp, err)
	}

	return edgeApplicationsResponse.Results, nil
}

func (c *Client) UpdateRulesEngine(ctx context.Context, req *UpdateRulesEngineRequest, idFunc int64) (EdgeApplicationsResponse, error) {

	request := c.apiClient.EdgeApplicationsRulesEngineApi.EdgeApplicationsEdgeApplicationIdRulesEnginePhaseRulesGet(ctx, req.IdApplication, "request")

	edgeApplicationRules, httpResp, err := request.Execute()
	if err != nil {
		return nil, utils.ErrorPerStatusCode(httpResp, err)
	}

	idRule := edgeApplicationRules.Results[0].Id

	b := make([]sdk.RulesEngineBehavior, 1)
	b[0].SetName("run_function")
	b[0].SetTarget(idFunc)
	req.SetBehaviors(b)

	requestUpdate := c.apiClient.EdgeApplicationsRulesEngineApi.EdgeApplicationsEdgeApplicationIdRulesEnginePhaseRulesRuleIdPatch(ctx, req.IdApplication, "request", idRule).PatchRulesEngineRequest(req.PatchRulesEngineRequest)

	edgeApplicationsResponse, httpResp, err := requestUpdate.Execute()
	if err != nil {
		return nil, utils.ErrorPerStatusCode(httpResp, err)
	}

	return &edgeApplicationsResponse.Results, nil
}
