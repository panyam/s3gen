
# Core Concepts

`s3gen`'s architecture is built around a few core concepts. Understanding these will help you to get the most out of the tool and build your site effectively.

## The `Site` Object

The `s3gen.Site` object is the central nervous system of your project. It's a Go struct that holds all the configuration for your website. You define your site's structure, build rules, and templating options here.

Here's a breakdown of some of the most important fields:

*   `ContentRoot`: The directory where all your content files are stored.
*   `OutputDir`: The directory where `s3gen` will write the generated static files.
*   `TemplateFolders`: A list of directories where your Go templates are located.
*   `BuildRules`: A slice of `Rule` objects that define how to process different types of files.
*   `PriorityFunc`: A function that determines the order in which resources are processed. This is crucial for handling dependencies.
*   `LiveReload`: A boolean that enables or disables live reloading during development.

## Resources

In `s3gen`, every file in your `ContentRoot` is treated as a `Resource`. A `Resource` is a Go struct that contains information about the file, including its path, content, front matter, and current state in the build process (e.g., `Pending`, `Loaded`, `Failed`).

This abstraction allows `s3gen` to work with different file types in a consistent way and provides a structured way to manage your content throughout the build process.

## Rules

A `Rule` is a Go interface that defines a set of actions to be performed on a `Resource`. It's the heart of `s3gen`'s extensibility. Each rule has two main methods:

*   `TargetsFor(site *Site, res *Resource)`: This method determines if the rule can be applied to a given resource and, if so, what the output file (or "target") should be.
*   `Run(site *Site, inputs []*Resource, targets []*Resource, ...)`: This method contains the logic for processing the resource. For example, the `MDToHtml` rule's `Run` method converts Markdown to HTML.

`s3gen` comes with a set of built-in rules for common tasks like processing Markdown and HTML files, but you can easily create your own to handle any file type you need.

## The Build Process

When you run a build, `s3gen` performs the following steps:

1.  **Discovery**: It walks your `ContentRoot` directory to find all the files and creates a `Resource` object for each one.
2.  **Prioritization**: It sorts the resources based on the `PriorityFunc` in your `Site` configuration. This is how it resolves dependencies, ensuring that, for example, all your blog posts are processed before the index page that lists them.
3.  **Rule Matching**: For each resource, it iterates through the `BuildRules` and uses the `TargetsFor` method to find a rule that can handle it.
4.  **Execution**: Once a matching rule is found, `s3gen` calls its `Run` method to process the resource and generate the output file(s).
5.  **Output**: The final HTML files and any other generated assets are written to the `OutputDir`.

This rule-based, prioritized build process gives you a great deal of flexibility and control over how your site is generated.
