// Command basic writes a single CAPTCHA to captcha.png.
package main

import (
	"fmt"
	"log"
	"os"

	nyancaptcha "github.com/mxmilu666/nyancaptcha"
)

func main() {
	g, err := nyancaptcha.New(nyancaptcha.WithDifficulty(nyancaptcha.Hard), nyancaptcha.WithLanguage(nyancaptcha.Chinese))
	if err != nil {
		log.Fatal(err)
	}
	c, err := g.Generate()
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile("captcha.png", c.Image, 0600); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Question: %s\nAnswer (server only): %s\nPNG: captcha.png\n", c.Question, c.Answer)
}
