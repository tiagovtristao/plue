#!/usr/bin/env node

const {parse} = require('@babel/parser');
const traverse = require('@babel/traverse').default;
const fs = require('fs');
const argv = require('minimist')(process.argv.slice(2));
const path = require('path');
const resolve = require('resolve');
const {createMatchPath, loadConfig} = require('tsconfig-paths');

// Your repo's module extension support and order
const EXTENSIONS = ['.js', '.ts', '.tsx'];

const PLZ_OUT = 'plz-out/';
const THIRD_PARTY_JS = 'plz-out/gen/third_party/js/';

function loadTsConfig(configFile) {
  const tsConfig = loadConfig(configFile);

  if (tsConfig.resultType !== 'success') {
    throw new Error(`Error loading TS config file: ${configFile}`);
  }
  
  return tsConfig;
}

function getFileImportIds(file) {
  const contents = fs.readFileSync(file, {encoding: 'utf8'});

  const ast = parse(contents, {
    sourceType: 'module',
    plugins: ['typescript', 'jsx'],
  });

  const dependencies = new Set();
  traverse(ast, {
    ImportDeclaration: (path) => {
      dependencies.add(path.node.source.value);
    },
  });

  return Array.from(dependencies);
}

function convertToCriteria(resolved, importId) {
  // Trim repo path
  const relativeResolved = resolved.substring(process.env.REPO.length + 1);

  if (relativeResolved.startsWith(THIRD_PARTY_JS)) {
    // Example: graphql-tag/lib/graphql-tag.umd.js
    const npmPackageImport = relativeResolved.substring(THIRD_PARTY_JS.length);

    let scope;
    let name;
    if (npmPackageImport.startsWith('@')) {
      [scope, name] = npmPackageImport.split('/');
    }
    else {
      scope = '';
      [name] = npmPackageImport.split('/');
    }
    
    return {
      type: 'package',
      importId,
      lookups: [{
        package: 'third_party/js' + (scope && `/${scope}`),
        call: {
          id: 'npm_library',
          args: {
            name: `^${name}$`
          },
          label: 'name'
        }
      }]
    };
  } else if (!relativeResolved.startsWith(PLZ_OUT)) {
    return {
      type: 'file',
      importId,
      lookup: {
        file: relativeResolved,
        calls: [
          {
            id: 'js_library',
            srcs: 'srcs',
            deps: 'deps',
            label: 'name',
          },
          {
            id: 'js_library',
            srcs: 'src',
            deps: 'deps',
            label: 'name',
          },
          {
            id: 'filegroup',
            srcs: 'srcs',
            deps: 'deps',
            label: 'name',
          },
        ]
      }
    };
  }
}

(function main() {
  const [file] = argv._;
  if (!file) {
    throw new Error('A file is required');
  }

  // Assuming there's a tsconfig.json at the root
  const tsConfigFile = path.join(process.env.REPO, 'tsconfig.json');
  const {absoluteBaseUrl, paths} = loadTsConfig(tsConfigFile);
  const matchPath = createMatchPath(absoluteBaseUrl, paths);

  const importIds = getFileImportIds(file);

  const importToResolved = importIds.reduce((acc, importId) => {
    // Resolve import id (clipping the extension), if it matches any of the compilerOptions.paths keys
    let resolved = matchPath(importId, undefined, undefined, EXTENSIONS);

    try {
      // Add extension back if import id was found above, otherwise tries to resolve it the node way
      resolved = resolve.sync(resolved || importId, {basedir: path.dirname(file), extensions: EXTENSIONS});
    }
    catch {
      throw new Error(`Unable to resolve import id: ${importId}`);
    }

    acc[importId] = resolved;

    return acc;
  }, {});

  const criteriaList = Object.entries(importToResolved).map(([importId, resolved]) => convertToCriteria(resolved, importId)).filter(Boolean);

  console.log(JSON.stringify(criteriaList));
})();

