// Пакет buildinfo отвечает за вывод в стандартный поток метаданных сборки - buildVersion, buildDate и buildCommit.
package buildinfo

import "fmt"

// Print выводит в стандартный поток метаданные сборки приложения, передаваемые в параметрах.
func Print(buildVersion, buildDate, buildCommit string) {
	if buildVersion == "" {
		buildVersion = "N/A"
	}
	fmt.Printf("Build version: %s\n", buildVersion)

	if buildDate == "" {
		buildDate = "N/A"
	}
	fmt.Printf("Build date: %s\n", buildDate)

	if buildCommit == "" {
		buildCommit = "N/A"
	}
	fmt.Printf("Build commit: %s\n", buildCommit)
}
