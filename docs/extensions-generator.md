# Writing PHP Extensions in Go

FrankenPHP is bundled with a tool that allows you **to create a PHP extension** only using Go.
**No need to write C code ** or use CGO directly: FrankenPHP also includes a **public types API**
to help you write your extensions in Go without having to worry about
**the type juggling between PHP/C and Go**.

> [!TIP]
> If you want to understand how extensions can be written in Go from scratch, you can read the
> dedicated page of the [FrankenPHP documentation](extensions.md) demonstrating how to write a
> PHP extension in Go without using the generator.

Keep in mind that this tool is **not a full-fledged extension generator**. It is meant to help you write simple
extensions in Go, but it does not provide the most advanced features of PHP extensions. If you need to write a more 
**complex and optimized** extension, you may need to write some C code or use CGO directly.

## Creating a Native Function

We will first see how to create a new native function in Go that can be called from PHP.

### Prerequisites

The first thing to do is to [get the PHP sources](https://www.php.net/downloads.php) before going further. Once you have
them, decompress them in the directory of your choice:

```console
tar xf php-*
```

Keep in mind the directory where you decompressed the PHP sources, as you will need it later. You can now create a new
Go module in the directory of your choice:

```console
go mod init github.com/my-account/my-module
```

### Writing the Extension

Everything is now setup to write your native function in Go. Create a new file named `stringext.go`. Our first function
will take a string as an argument, the number of times to repeat it, a boolean to indicate whether to reverse the
string, and return the resulting string. This should look like this:

```go
import (
    "C"
    "github.com/dunglas/frankenphp/types"
    "strings"
)

//export_php repeat_this(string $str, int $count, bool $reverse): string
func repeat_this(s *C.zend_string, count int64, reverse bool) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(s))

    result := strings.Repeat(str, int(count))
    if reverse {
        runes := []rune(result)
        for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
            runes[i], runes[j] = runes[j], runes[i]
        }
        result = string(runes)
    }

    return frankenphp.PHPString(result, false)
}
```

There are two important things to note here:

* A directive comment `//export_php` defines the function signature in PHP. This is how the generator knows how to
  generate the PHP function with the right parameters and return type.
* The function must return an `unsafe.Pointer`. FrankenPHP provides an API to help you with type juggling between C and
  Go.

While the first point speaks for itself, the second may be harder to apprehend. Let's take a deeper dive to type
juggling in the next section.

## Type Juggling

While some variable types have the same memory representation between C/PHP and Go, some types require more logic to be
directly used. This is maybe the hardest part when it comes to writing extensions because it requires understanding
internals of the Zend Engine and how variables are stored internally in PHP. This table summarizes what you need to
know:

| PHP type | Go type       | Direct conversion | C to Go helper        | Go to C helper         |
|----------|---------------|-------------------|-----------------------|------------------------|
| `int`    | `int64`       | ✅                 | -                     | -                      |
| `float`  | `float64`     | ✅                 | -                     | -                      | 
| `bool`   | `bool`        | ✅                 | -                     | -                      |
| `string` | `string`      | ❌                 | frankenphp.GoString() | frankenphp.PHPString() |
| `array`  | `slice`/`map` | ❌                 | _Not yet implemented_ | _Not yet implemented_  |
| `object` | `struct`      | ❌                 | _Not yet implemented_ | _Not yet implemented_  |

> [!NOTE]
> This table is not exhaustive yet and will be completed as the FrankenPHP types API gets more complete.

If you refer to the code snippet of the previous section, you can see that helpers are used to convert the first
parameter and the return value. The second and third parameter of our `repeat_this()` function don't need to be
converted as memory representation of the underlying types are the same for both C and Go.

## Declaring a Native PHP Class

The generator also supports declaring classes as Go structs, which can be used to create PHP objects. You can use the
`// export_php` directive comment again to define a PHP class. For example:

```go
// export_php: class FrankenPhp
type FrankenPhpGoStruct struct {
    Name       string
    Type       int
    IsNullable *bool
}
```

That's it. The generator will automatically generate the PHP class with the properties defined in the Go struct. You can
then use this class in your PHP code.

## Generating the Extension

This is where the magic happens, and your extension can now be generated. You can run the generator with the following
command:

```console
GEN_STUB_FILE=php-src/build/gen_stub.php frankenphp extension-init my_extension.go 
```

> [!NOTE]
> Don't forget to set the `GEN_STUB_FILE` environment variable to the path of the `gen_stub.php` file in the PHP
> sources. This file is used by the generator to create the PHP function stubs.

If everything went well, a new directory named `build` should have been created. This directory contains the generated
files for your extension, including the `my_extension.go` file with the generated PHP function stubs.

### Integrate the Extension into FrankenPHP

Our extension is now ready to be compiled and integrated into FrankenPHP. To do this, refer to the
FrankenPHP [compilation documentation](compile.md) to learn how to compile FrankenPHP. The only difference is that you
need to add your Go module to the compilation command. Using `xcaddy`, it will look like this:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

All that's left is to create a PHP file to test the extension. For example, create an `index.php` file with the
following content:

```php
<?php

var_dump(repeat_this("Hello World!", 4, true));
```

You can now run FrankenPHP with this file using `./frankenphp php-server`, and you should see the message
`string(48) "!dlroW olleH!dlroW olleH!dlroW olleH!dlroW olleH"` on your screen.
