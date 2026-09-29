package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/usememos/memos/cmd/memos/backupdata"
)

func newBackupCommand() *cobra.Command {
	root := &cobra.Command{Use: "backup", Short: "Create or restore an offline SQLite instance backup"}
	var dataDir, dbName, output, input, target string
	create := &cobra.Command{Use: "create", Short: "Back up a stopped SQLite instance", RunE: func(_ *cobra.Command, _ []string) error {
		manifest, err := backupdata.Create(context.Background(), dataDir, dbName, output)
		if err != nil {
			return err
		}
		fmt.Printf("Backup verified: %d files at %s\n", len(manifest.Files), output)
		return nil
	}}
	create.Flags().StringVar(&dataDir, "data", "", "instance data directory")
	create.Flags().StringVar(&dbName, "db-name", "memos_prod.db", "SQLite database filename")
	create.Flags().StringVar(&output, "output", "", "new backup directory")
	_ = create.MarkFlagRequired("data")
	_ = create.MarkFlagRequired("output")
	restore := &cobra.Command{Use: "restore", Short: "Restore into a new data directory", RunE: func(_ *cobra.Command, _ []string) error {
		manifest, err := backupdata.Restore(context.Background(), input, target)
		if err != nil {
			return err
		}
		fmt.Printf("Restore verified: %d files at %s\n", len(manifest.Files), target)
		return nil
	}}
	restore.Flags().StringVar(&input, "input", "", "backup directory")
	restore.Flags().StringVar(&target, "target", "", "new restore directory")
	_ = restore.MarkFlagRequired("input")
	_ = restore.MarkFlagRequired("target")
	root.AddCommand(create, restore)
	return root
}
