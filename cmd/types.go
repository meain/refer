package cmd

type CLI struct {
	Add     Add      `cmd:"" help:"Add a file or directory to the database"`
	Search  Search   `cmd:"" help:"Search for documents"`
	Show    Show     `cmd:"" help:"List documents in the database"`
	Stats   StatsCmd `cmd:"" help:"Show database statistics"`
	Reindex Reindex  `cmd:"" help:"Reindex all documents"`
	Remove  Remove   `cmd:"" help:"Remove a document from the database"`
}

type Add struct {
	FilePath  string `kong:"arg,required"`
	Recursive bool   `kong:"help='Recursive'"`
}

type Search struct {
	Query  string `kong:"arg,required"`
	Format string `kong:"default='names'"`
	Limit  int    `kong:"default=5"`
}

type Reindex struct{}

type Show struct {
	ID *int `arg:"" optional:"" help:"Optional document ID to show details for a specific document"`
}

type StatsCmd struct{}

type Remove struct {
	ID int `arg:"" help:"Document ID to remove"`
}
