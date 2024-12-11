package cmd

type CLI struct {
	Add    Add    `kong:"cmd"`
	Search Search `kong:"cmd"`
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
