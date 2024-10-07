package main

import (
	"fmt"
	"os"

	"github.com/andy1truong/veilvault/internal/veilvault"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "veilvault",
		Short: "VeilVault: A tool to encode and decode directories to PNG images",
		Long:  `VeilVault allows you to encode a directory into a PNG file and decode a PNG file back into a directory, securely hiding your data.`,
	}

	var encodeCmd = &cobra.Command{
		Use:   "encode [dirPath] [imagePath]",
		Short: "Encode a directory into a PNG file",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			dirPath := args[0]
			imagePath := args[1]
			password, _ := cmd.Flags().GetString("password")

			err := veilvault.Encode(dirPath, imagePath, password)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding directory: %v\n", err)
			} else {
				fmt.Println("Directory encoded successfully into PNG!")
			}
		},
	}

	var decodeCmd = &cobra.Command{
		Use:   "decode [imagePath] [outputDir]",
		Short: "Decode a PNG file back into a directory",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			imagePath := args[0]
			outputDir := args[1]
			password, _ := cmd.Flags().GetString("password")

			err := veilvault.Decode(imagePath, outputDir, password)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding PNG file: %v\n", err)
			} else {
				fmt.Println("PNG file decoded successfully into directory!")
			}
		},
	}

	// Add password flag for both commands
	encodeCmd.Flags().StringP("password", "p", "", "Password for encoding")
	decodeCmd.Flags().StringP("password", "p", "", "Password for decoding")

	rootCmd.AddCommand(encodeCmd)
	rootCmd.AddCommand(decodeCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
