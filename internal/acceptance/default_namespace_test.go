package acceptance

import (
	"context"
	"errors"
	"testing"

	"github.com/databricks/databricks-sdk-go/apierr"
	"github.com/databricks/databricks-sdk-go/service/settings"
	"github.com/databricks/terraform-provider-databricks/common"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccDefaultNamespaceSetting(t *testing.T) {
	template := `
	resource "databricks_default_namespace_setting" "this" {
		namespace {
			value = "namespace_value"
		}
	}
	`
	workspaceLevel(t, step{
		Template: template,
		Check: resourceCheckWithState("databricks_default_namespace_setting.this",
			func(ctx context.Context, client *common.DatabricksClient, state *terraform.InstanceState) error {
				ctx = context.WithValue(ctx, common.Api, common.API_2_1)
				w, err := client.WorkspaceClient()
				require.NoError(t, err)
				etag := state.Attributes["etag"]
				require.NotEmpty(t, etag)
				res, err := w.Settings.GetDefaultNamespaceSetting(ctx, settings.GetDefaultNamespaceSettingRequest{
					Etag: etag,
				})
				require.NoError(t, err)
				// Check that the resource has been created and that it has the correct value.
				assert.Equal(t, res.Namespace.Value, "namespace_value")
				return nil
			}),
	},
		step{
			Template: template,
			Destroy:  true,
			Check: resourceCheck("databricks_default_namespace_setting.this", func(ctx context.Context, client *common.DatabricksClient, id string) error {
				ctx = context.WithValue(ctx, common.Api, common.API_2_1)
				w, err := client.WorkspaceClient()
				require.NoError(t, err)
				// Terraform Check returns the latest resource status before it is destroyed, which has an outdated eTag.
				// We are making an update call to get the correct eTag in the response error.
				_, err = w.Settings.UpdateDefaultNamespaceSetting(ctx, settings.UpdateDefaultNamespaceSettingRequest{
					AllowMissing: true,
					Setting: settings.DefaultNamespaceSetting{
						Namespace: settings.StringMessage{
							Value: "this_call_should_fail",
						},
					},
					FieldMask: "namespace.value",
				})
				assert.Error(t, err)
				var aerr *apierr.APIError
				if !errors.As(err, &aerr) {
					assert.FailNow(t, "cannot parse error message %v", err)
				}
				etag := aerr.Details[0].Metadata["etag"]
				_, err = w.Settings.GetDefaultNamespaceSetting(ctx, settings.GetDefaultNamespaceSettingRequest{
					Etag: etag,
				})
				if !errors.As(err, &aerr) {
					assert.FailNow(t, "cannot parse error message %v", err)
				}

				assert.Equal(t, aerr.ErrorCode, "RESOURCE_DOES_NOT_EXIST")
				return nil
			}),
		},
	)
}
