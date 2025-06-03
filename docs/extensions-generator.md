# Writing PHP Extensions in Go

FrankenPHP is bundled with a tool that allows you **to create a PHP extension** only using Go. **No need to write C code** or use CGO directly: FrankenPHP also includes a **public types API** to help you write your extensions in Go without having to worry about **the type juggling between PHP/C and Go**.

> [!TIP]
> If you want to understand how extensions can be written in Go from scratch, you can read the
> dedicated page of the [FrankenPHP documentation](extensions.md) demonstrating how to write a
> PHP extension in Go without using the generator.

Keep in mind that this tool is **not a full-fledged extension generator**. It is meant to help you write simple extensions in Go, but it does not provide the most advanced features of PHP extensions. If you need to write a more **complex and optimized** extension, you may need to write some C code or use CGO directly.

## Creating a Native Function

We will first see how to create a new native function in Go that can be called from PHP.

### Prerequisites

The first thing to do is to [get the PHP sources](https://www.php.net/downloads.php) before going further. Once you have them, decompress them into the directory of your choice:

```console
tar xf php-*
```

Keep in mind the directory where you decompressed the PHP sources, as you will need it later. You can now create a new Go module in the directory of your choice:

```console
go mod init github.com/my-account/my-module
```

### Writing the Extension

Everything is now setup to write your native function in Go. Create a new file named `my_extension.go`. Our first function will take a string as an argument, the number of times to repeat it, a boolean to indicate whether to reverse the string, and return the resulting string. This should look like this:

```go
import (
    "C"
    "github.com/dunglas/frankenphp/types"
    "strings"
)

//export_php: repeat_this(string $str, int $count, bool $reverse): string
func repeat_this(s *C.go_string, count C.long, reverse C.int) *C.go_value {
	data := types.CStringToGoString(s)

	result := strings.Repeat(data, int(count))
	if reverse != 0 {
		runes := []rune(result)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		result = string(runes)
	}

	return types.String(result).CValue
}
```

There are two important things to note here:

 * A directive comment `//export_php` defines the function signature in PHP. This is how the generator knows how to generate the PHP function with the right parameters and return type.
 * The function must return a `*C.go_value`, which is the type used by the generator to represent PHP values in Go. You can use the `types` package to convert Go values to PHP values. Here, we use `types.String(result)` to convert the Go string to a PHP string. Note that the `CValue` field is used to get the `*C.go_value` representation.

FrankenPHP provides a set of types in the `types` package to help you convert between Go and PHP types. This way, you can still write your extension in Go without having to worry about the underlying C types. The `types` packages contains the following types:

 * `types.String()`: Converts a Go string to a PHP string;
 * `types.Int()`: Converts a Go int to a PHP int;
 * `types.Float()`: Converts a Go float64 to a PHP float;
 * `types.Bool()`: Converts a Go bool to a PHP bool;
 * `types.Array()`: Converts a Go slice to a PHP array;
 * `types.Map()`: Converts a Go map to a PHP associative array (a.k.a. hashmaps);
 * `types.Null()`: Returns a PHP null;
 * `types.Object()`: Converts a Go struct to a PHP object, the second argument being the name of the class.

The package also provides a set of functions to convert PHP values to Go values:

 * `types.CStringToGoString()`: Converts a PHP string to a Go string;
 * `types.PhpArrayToSlice()`: Converts a PHP array to a Go slice;
 * `types.PhpArrayToMap()`: Converts a PHP hashmap to a Go map;
 * `types.PhpObjectToMap()`: Converts a PHP object to a Go map;
 * `types.IsAssociativeArray()`: Checks if a provided PHP array is associative;
 * `types.Nullable*`: Converts a PHP value to a Go value, returning a pointer to the value or nil if the value is null. This is particularly useful when you handle nullable parameters in your functions.

## Declaring a Native PHP Class

The generator also supports declaring classes as Go structs, which can be used to create PHP objects. You can use the `// php_class` directive comment to define a class in Go. For example:

```go
// php_class: FrankenPhp
type FrankenPhpGoStruct struct {
	Name       string
	Type       int
	IsNullable *bool
}
```

That's it. The generator will automatically generate the PHP class with the properties defined in the Go struct. You can then use this class in your PHP code.

## Generating the Extension

This is where the magic happens, and your extension can now be generated. You can run the generator with the following command:

```console
GEN_STUB_FILE=php-src/build/gen_stub.php frankenphp extension-init my_extension.go 
```

> [!NOTE]
> Don't forget to set the `GEN_STUB_FILE` environment variable to the path of the `gen_stub.php` file in the PHP sources. This file is used by the generator to create the PHP function stubs.

If everything went well, a new directory named `build` should have been created. This directory contains the generated files for your extension, including the `my_extension.go` file with the generated PHP function stubs.

### Integrate the Extension into FrankenPHP

Our extension is now ready to be compiled and integrated into FrankenPHP. To do this, refer to the FrankenPHP [compilation documentation](compile.md) to learn how to compile FrankenPHP. The only difference is that you need to add your Go module to the compilation command. Using `xcaddy`, it will look like this:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

All that's left is to create a PHP file to test the extension. For example, create an `index.php` file with the following content:

```php
<?php

var_dump(repeat_this("Hello World!", 4, true));
```

You can now run FrankenPHP with this file using `./frankenphp php-server`, and you should see the message `string(48) "!dlroW olleH!dlroW olleH!dlroW olleH!dlroW olleH"` on your screen.
