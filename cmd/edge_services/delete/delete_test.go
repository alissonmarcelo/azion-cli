package delete

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/aziontech/azion-cli/pkg/cmdutil"
	"github.com/aziontech/azion-cli/pkg/httpmock"
	"github.com/aziontech/azion-cli/pkg/iostreams"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	t.Run("delete service by id", func(t *testing.T) {
		mock := &httpmock.Registry{}

		mock.Register(
			httpmock.REST("DELETE", "edge_services/1234"),
			httpmock.StatusStringResponse(204, ""),
		)

		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		f := &cmdutil.Factory{
			HttpClient: func() (*http.Client, error) {
				return &http.Client{Transport: mock}, nil
			},
			IOStreams: &iostreams.IOStreams{
				Out: stdout,
				Err: stderr,
			},
			Config: viper.New(),
		}

		cmd := NewCmd(f)
		cmd.PersistentFlags().BoolP("verbose", "v", false, "")
		cmd.SetArgs([]string{"1234"})
		cmd.SetIn(&bytes.Buffer{})
		cmd.SetOut(ioutil.Discard)
		cmd.SetErr(ioutil.Discard)

		_, err := cmd.ExecuteC()
		require.NoError(t, err)

		assert.Equal(t, "", stdout.String())
	})

	t.Run("delete service by id being verbose", func(t *testing.T) {
		mock := &httpmock.Registry{}

		mock.Register(
			httpmock.REST("DELETE", "edge_services/1234"),
			httpmock.StatusStringResponse(204, ""),
		)

		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		f := &cmdutil.Factory{
			HttpClient: func() (*http.Client, error) {
				return &http.Client{Transport: mock}, nil
			},
			IOStreams: &iostreams.IOStreams{
				Out: stdout,
				Err: stderr,
			},
			Config: viper.New(),
		}

		cmd := NewCmd(f)
		cmd.PersistentFlags().BoolP("verbose", "v", false, "")
		cmd.SetArgs([]string{"1234", "-v"})
		cmd.SetIn(&bytes.Buffer{})
		cmd.SetOut(ioutil.Discard)
		cmd.SetErr(ioutil.Discard)

		_, err := cmd.ExecuteC()
		require.NoError(t, err)

		assert.Equal(t, "Service 1234 was successfully deleted\n", stdout.String())
	})

	t.Run("delete service that is not found", func(t *testing.T) {
		mock := &httpmock.Registry{}

		mock.Register(
			httpmock.REST("DELETE", "edge_services/1234"),
			httpmock.StatusStringResponse(404, "Not Found"),
		)

		stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
		f := &cmdutil.Factory{
			HttpClient: func() (*http.Client, error) {
				return &http.Client{Transport: mock}, nil
			},
			IOStreams: &iostreams.IOStreams{
				Out: stdout,
				Err: stderr,
			},
			Config: viper.New(),
		}

		cmd := NewCmd(f)

		cmd.SetArgs([]string{"1234"})
		cmd.SetIn(&bytes.Buffer{})
		cmd.SetOut(ioutil.Discard)
		cmd.SetErr(ioutil.Discard)

		_, err := cmd.ExecuteC()
		require.Error(t, err)
	})
}
