# Plue

An experiment for achieving some automation between sources in a [Please](https://please.build) repo and their build rules.

## Motivation

When working with **Please**, your sources need to be declared in build rules, along with their respective dependencies, so that they are built as expected. However, the maintenance of these rules affect the developer experience to varying degrees.

Also, dev tools and parsers are interesting :-)

## Automation

Ideally, you would rarely need to manually deal with **BUILD** files. This would be even more applicable to projects (in the repo) where there's already a pattern for how to create/update the relevant build rules. For instance, if your **Go** packages are always defined by a single rule, then a new file/source in that directory could be automatically added to the existing rule, or a new `go_library` rule could be automatically created with that new source, if it didn't exist already. Something similar could also be achieved for **JavaScript** projects, where in this case there's usually a single build rule for each file (i.e. `js_library`).
This automation could also be extended to dealing with the files' dependencies, where imports would get mapped to their targets, and listed as dependencies in the files' build rules.  

Keeping the dependencies between files and build rules in sync is arguably the most time consuming aspect of maintaining **BUILD** files when developing code personally, and therefore a good first candidate for automation.

## Missing dependencies

Given that the scope discussed above not only is broad but its feasibility is also unknown at this point (I'm not an expert in build tools), I set out to experiment if there could be an automated way of working out missing dependencies on build targets against those imported from their sources. This would reduce the initial scope and provide more insight into what's possible.

### Current implementation

Let's say you have the `common/js/foobar/myLib.js` file with the following imports:

```javascript
import fs from 'fs';
import React from 'react';
import {max} from '../util';
```

To find out any missing dependencies, you run:
`go run main.go --repo-config example/repo.json missing-deps common/js/foobar/myLib.js`

> Your `--repo-config` file needs to be set up properly. More on it below.

The output of the command run above would look something like the following:
```
# Source file's BUILD listed dependency targets:
//third_party/js:react

# Source files's missing dependency targets:
"//common/js/util:index",
```

(At the moment, you would have to select the missing dependencies listed and paste them onto the build rule of the file.)

### Repo configuration

You need to provide a JSON configuration file through the `--repo-config` flag about the repo that you are acting on. In this case, the repo where you are trying to find out any missing build dependencies for a given source file.

Configuration example:
```json
{
  "repo": "/full/path/to/repo",
  "extensionsConfig": {
    ".js": {
      "sourceFileCriteriaLookup": [{
        "id": "js_library",
        "srcs": "srcs",
        "deps": "deps",
        "label": "name"
      }],
      "depsResolver": "./example/js/resolver.js"
    }
  }
}
```

The `sourceFileCriteriaLookup` key is used by the command ran earlier to find the rule that includes `common/js/foobar/myLib.js` as a source - at the moment, the file's rule is expected to already exist. So the above configuration will look into the `js_library` call including this file. This will allow to extract the already listed dependencies in that rule.

More than one lookup can be provided, where the first matching instance will be used.

- `id` refers to the function call name
- `srcs` is the call argument for the sources
- `deps` is the call argument for the dependencies
- `label` is the call argument for the build rule name

#### Dependencies resolver

The `depsResolver` key has a program path as value that will be executed. And following the example above, it would be called as: `REPO=/full/path/to/repo ./example/js/resolver.js common/js/foobar/myLib.js`.
This program is responsible for reading the imports in the file and report back how their targets can be found, by returning a JSON-stringified list of search criteria.

At the moment, 2 lookup criteria options are supported.

##### `file` lookup criteria

For the `import {max} from '../util';` example above, this could translated into something similar to:

```jsonc
{
  "type": "file",
  "importId": "../util",
  "lookup": {
    // This could be a node module resolution of `../util`
    "file": "common/js/util/index.js",
    // You can specify all possible calls where the file can be found as a source
    "calls": [
      {
        "id": "js_library",
        "srcs": "srcs",
        "deps": "deps",
        "label": "name"
      }
    ]
  }
}
```

This lookup information for `../util` returned by the resolver is used by **Plue** to find a `js_library` call that includes `common/js/utils/index.js` as a source. Although the `deps` key isn't used at this point, both `srcs` and `label` are used to know where to look for the source and extract the rule label respectively.

##### `package` lookup criteria

For the `import React from 'react';` example above, this could translated into something similar to:

```json
{
  "type": "package",
  "importId": "react",
  "lookups": [{
    "package": "third_party/js",
    "call": {
      "id": "npm_library",
      "args": {
        "name": "^react$"
      },
      "label": "name"
    }
  }]
}
```

This lookup information for `react` returned by the resolver is used by **Please** to find a `npm_libray` call in the `third_party/js` package that has the `name` argument equal to `react`. All the `args` values are expected to be regular expressions, and a match is found when all of them are true.

### Design decisions

You tell **Plue** how to find the build targets for a file's imports/dependencies.

- You know where the dependencies' targets are (or to some extent):
  -  For single file dependencies, for instance, they might be behind a `filegroup` or `js_library` call and that's most of what you need to tell **Plue** where to find it, by using the **`file` lookup criteria**.
  -  For internal libraries, they might be found in the same directories where the dependencies are being imported from. For external libraries, for instance, they might be always found in the same directory (i.e. `third_party/go`). This is where the **`package` lookup criteria** would be used.
- Dependencies' targets are assumed to be defined via **Please**'s **asp** function call language construct.
- It should be flexible to whatever language you might be using. Hence why the `depsResolver` program in the configuration file is defined by file extension.
  - There's language-specific tooling when it comes to the parsing of import statements and what they resolve to.
  - You have finer control over the lookup criteria list you generate. For instance, different `Go` projects (in the same repo) might be set up differently or use different build rules.
  - You can leverage it to enforce some standards for the management of dependencies (i.e. if a source imports a dependency, its target should be included directly on the source's rule, instead of being made available through a transitive dependency).

### Dependencies

This project currently depends on a forked version of **Please**.

One of the design decisions was to assume that dependencies' targets are defined via **Please**'s **asp** function call language construct. But performing static analysis on **BUILD** files wouldn't be enough to extract all the information required, which is guaranteed to be available at runtime. For instance, you might have a defined target where the `srcs` argument specifices a `glob` expression (or any other expression) that needs to be evaluated first to render more concrete and primitive values.

This fork provides a hook into the stage where **asp**'s interpreter has finished initialising function calls before executing their statements. Accessing these snapshots of evaluated call arguments allows the **lookup criteria** (mentioned above) to be possible.

### Outcome

Good initial results were achieved in obtaining missing dependencies' targets for different file sources in different languages.
Initial boilerplate for `depsResolver` programs (you can find some examples in the **example** directory) should be similar (i.e. parsing import statements and generating the lookup criteria list) across same language projects and different repos. They would just need to be further configured to the repo's use cases.

The **Please** fork has proven to be unmaintainable and unscalable:
- There's are a few steps that need to be manually performed to guarantee a sucessful merge/rebase of a recent upstream master
- Your **Please** repo will likely be using a different version than this fork uses, which might cause **Plue** to error out.

## Can I try it?

1. There's a sample repo configuration file and a couple of resolvers in the **example** directory:
1.1. The `Go` resolver is a pretty basic one that might need little tweaking for a standard **Please** repo.
1.2. The `JS` resolver is more complex since it is setup for TypeScript repos with path mappings configured.
2. Cross your fingers and hope your **Please** version is compatible :-) 

## Future

As stated above, the **Please** fork is a major concern at this point, although initial promising results were shown. It's also early to say the extent of results/automation that could be achieved.

I don't have any plans on investigating this any further due to lack of free time, but I'm sharing this experiment in case it might be useful to someone trying to achieve something similar (either in **Please** or other build tools). 

