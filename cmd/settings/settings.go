package settings

type Config struct {
	ServerAddress  string `env:"RUN_ADDRESS" envDefault:"localhost:8080"`
	DatabaseURI    string `env:"DATABASE_URI" envDefault:"postgres://postgres:example@localhost:5432"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS" envDefault:""`
}
