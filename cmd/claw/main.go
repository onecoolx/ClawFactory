// ClawFactory CLI tool entry point.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	baseURL    string
	apiToken   string
	outputJSON bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "claw",
		Short: "ClawFactory CLI - multi-agent orchestration platform command-line tool",
	}

	rootCmd.PersistentFlags().StringVar(&baseURL, "url", "http://localhost:8080", "ClawFactory server address")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "dev-token-001", "API Token")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "output-json", false, "output in JSON format")

	// workflow command group
	workflowCmd := &cobra.Command{Use: "workflow", Short: "Workflow management"}
	workflowCmd.AddCommand(workflowSubmitCmd(), workflowStatusCmd(), workflowArtifactsCmd())

	// agent command group
	agentCmd := &cobra.Command{Use: "agent", Short: "Agent management"}
	agentCmd.AddCommand(agentListCmd(), agentLogsCmd())

	rootCmd.AddCommand(workflowCmd, agentCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to ClawFactory service (%s): %w", baseURL, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func workflowSubmitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "submit [workflow.json]",
		Short: "Submit a workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			var def map[string]interface{}
			if err := json.Unmarshal(data, &def); err != nil {
				return fmt.Errorf("failed to parse JSON: %w", err)
			}
			resp, err := doRequest("POST", "/v1/admin/workflows", def)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var result map[string]interface{}
				json.Unmarshal(resp, &result)
				fmt.Printf("Workflow submitted: %v\n", result["instance_id"])
			}
			return nil
		},
	}
}

func workflowStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [workflow_id]",
		Short: "Query workflow status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/workflows/"+args[0], nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var result map[string]interface{}
				json.Unmarshal(resp, &result)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "ID\t%v\n", result["instance_id"])
				fmt.Fprintf(w, "Status\t%v\n", result["status"])
				w.Flush()
			}
			return nil
		},
	}
}

func workflowArtifactsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "artifacts [workflow_id]",
		Short: "Query workflow artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/workflows/"+args[0]+"/artifacts", nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var artifacts []map[string]interface{}
				json.Unmarshal(resp, &artifacts)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "WORKFLOW\tTASK\tNAME\tPATH\n")
				for _, a := range artifacts {
					fmt.Fprintf(w, "%v\t%v\t%v\t%v\n", a["workflow_id"], a["task_id"], a["name"], a["path"])
				}
				w.Flush()
			}
			return nil
		},
	}
}

func agentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/agents", nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var agents []map[string]interface{}
				json.Unmarshal(resp, &agents)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tNAME\tSTATUS\tVERSION\tCAPABILITIES\n")
				for _, a := range agents {
					fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
						a["agent_id"], a["name"], a["status"], a["version"], a["capabilities"])
				}
				w.Flush()
			}
			return nil
		},
	}
}

func agentLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [agent_id]",
		Short: "Query agent logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/v1/admin/agents/"+args[0]+"/logs", nil)
			if err != nil {
				return err
			}
			if outputJSON {
				fmt.Println(string(resp))
			} else {
				var logs []map[string]interface{}
				json.Unmarshal(resp, &logs)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "TIMESTAMP\tLEVEL\tTASK\tMESSAGE\n")
				for _, l := range logs {
					fmt.Fprintf(w, "%v\t%v\t%v\t%v\n",
						l["timestamp"], l["level"], l["task_id"], l["message"])
				}
				w.Flush()
			}
			return nil
		},
	}
}
