package writer

import (
	"fmt"
	"os"

	"github.com/dave/jennifer/jen"

	"github.com/mikekonan/go-oas3/configurator"
	"github.com/mikekonan/go-oas3/generator"
)

type Writer struct {
	config *configurator.Config `di.inject:"config"`
}

func (writer *Writer) Write(result *generator.Result) error {
	if err := writer.checkDirs(); err != nil {
		return err
	}

	if err := writer.write(writer.config.Path, result.RouterCode); err != nil {
		return err
	}

	if err := writer.write(writer.config.ComponentsPath, result.ComponentsCode); err != nil {
		return err
	}

	return nil
}

func (writer *Writer) write(into string, code *jen.File) error {
	file, err := os.OpenFile(writer.config.Path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)

	if err != nil {
		return fmt.Errorf("failed opening file '%s': %v", into, err)
	}

	if err := code.Render(file); err != nil {
		return fmt.Errorf("failed rending into file '%s': %v", into, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed closing file '%s': %v", into, err)
	}

	return nil
}

func (writer *Writer) checkDirs() error {
	isDir, err := writer.isDir(writer.config.Path)
	if err != nil {
		return fmt.Errorf("failed checking dir '%s': %v", writer.config.Path, err)
	}

	if !isDir {
		return fmt.Errorf("failed checking dir '%s': not directory", writer.config.Path)
	}

	isDir, err = writer.isDir(writer.config.ComponentsPath)
	if err != nil {
		return fmt.Errorf("failed checking dir '%s': %v", writer.config.ComponentsPath, err)
	}

	if !isDir {
		return fmt.Errorf("failed checking dir '%s': not directory", writer.config.ComponentsPath)
	}

	return nil
}

func (writer *Writer) isDir(path string) (bool, error) {
	file, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return file.IsDir(), nil

}
