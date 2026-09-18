const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

const OPENAPI_FILE = path.join(__dirname, 'openapi.yaml');
const MODULAR_DIR = path.join(__dirname, 'modular');

function sanitizePathName(pathName) {
  let result = pathName;
  result = result.replace(/^\//, '');
  result = result.split('/').join('-');
  result = result.replace(/_/g, '-');
  result = result.replace(/--/g, '-');
  return result || 'root';
}

function modularizeOpenAPI() {
  console.log('Reading source OpenAPI file...');
  const content = fs.readFileSync(OPENAPI_FILE, 'utf8');
  const doc = yaml.load(content);
  
  if (fs.existsSync(MODULAR_DIR)) {
    fs.rmSync(MODULAR_DIR, { recursive: true });
  }
  
  const componentsDir = path.join(MODULAR_DIR, 'components');
  
  fs.mkdirSync(path.join(componentsDir, 'schemas'), { recursive: true });
  fs.mkdirSync(path.join(componentsDir, 'responses'), { recursive: true });
  fs.mkdirSync(path.join(componentsDir, 'securitySchemes'), { recursive: true });
  fs.mkdirSync(path.join(componentsDir, 'headers'), { recursive: true });
  fs.mkdirSync(path.join(MODULAR_DIR, 'paths'), { recursive: true });
  
  const schemas = doc.components?.schemas || {};
  const responses = doc.components?.responses || {};
  const securitySchemes = doc.components?.securitySchemes || {};
  const headers = doc.components?.headers || {};
  const paths = doc.paths || {};
  
  console.log(`Found ${Object.keys(schemas).length} schemas, ${Object.keys(paths).length} paths, ${Object.keys(responses).length} responses`);
  
  for (const [name, schemaDef] of Object.entries(schemas)) {
    const filePath = path.join(componentsDir, 'schemas', name + '.yaml');
    fs.writeFileSync(filePath, yaml.dump(schemaDef, { lineWidth: -1, indent: 2 }));
  }
  
  for (const [name, respDef] of Object.entries(responses)) {
    const filePath = path.join(componentsDir, 'responses', name + '.yaml');
    fs.writeFileSync(filePath, yaml.dump(respDef, { lineWidth: -1, indent: 2 }));
  }
  
  for (const [name, schemeDef] of Object.entries(securitySchemes)) {
    const filePath = path.join(componentsDir, 'securitySchemes', name + '.yaml');
    fs.writeFileSync(filePath, yaml.dump(schemeDef, { lineWidth: -1, indent: 2 }));
  }
  
  for (const [name, hdrDef] of Object.entries(headers)) {
    const filePath = path.join(componentsDir, 'headers', name + '.yaml');
    fs.writeFileSync(filePath, yaml.dump(hdrDef, { lineWidth: -1, indent: 2 }));
  }
  
  console.log('Extracting paths...');
  for (const [pathName, methods] of Object.entries(paths)) {
    const sanitizedPath = sanitizePathName(pathName);
    const filePath = path.join(MODULAR_DIR, 'paths', sanitizedPath + '.yaml');
    fs.writeFileSync(filePath, yaml.dump(methods, { lineWidth: -1, indent: 2 }));
  }
  
  console.log('Creating main openapi.yaml...');
  
  const openapiLines = [];
  openapiLines.push('openapi: 3.0.3');
  openapiLines.push('info:');
  openapiLines.push('  title: ' + (doc.info.title || 'CIVORA API'));
  openapiLines.push('  description: >-');
  for (const line of doc.info.description.split('\n')) {
    openapiLines.push('    ' + line.trimStart());
  }
  openapiLines.push('  version: ' + (doc.info.version || '0.8.0'));
  if (doc.info.contact) {
    openapiLines.push('  contact:');
    openapiLines.push('    name: ' + (doc.info.contact.name || 'CIVORA'));
    if (doc.info.contact.url) {
      openapiLines.push('    url: ' + doc.info.contact.url);
    }
  }
  if (doc.info.license) {
    openapiLines.push('  license:');
    openapiLines.push('    name: ' + (doc.info.license.name || 'Apache-2.0'));
    if (doc.info.license.url) {
      openapiLines.push('    url: ' + doc.info.license.url);
    }
  }
  openapiLines.push('');
  openapiLines.push('servers:');
  for (const server of (doc.servers || [])) {
    openapiLines.push('  - url: ' + server.url);
    if (server.description) {
      openapiLines.push('    description: ' + server.description);
    }
  }
  openapiLines.push('security: []');
  openapiLines.push('tags:');
  for (const tag of (doc.tags || [])) {
    openapiLines.push('  - name: ' + tag.name);
    if (tag.description) {
      openapiLines.push('    description: ' + tag.description);
    }
  }
  openapiLines.push('');
  openapiLines.push('components:');
  openapiLines.push('  securitySchemes:');
  openapiLines.push('    BearerAuth:');
  openapiLines.push('      $ref: ./modular/components/securitySchemes/BearerAuth.yaml');
  openapiLines.push('  headers:');
  openapiLines.push('    X-Request-ID:');
  openapiLines.push('      $ref: ./modular/components/headers/X-Request-ID.yaml');
  openapiLines.push('  responses:');
  for (const name of ['BadRequest', 'Unauthorized', 'Forbidden', 'NotFound', 'Conflict', 'RequestTooLarge']) {
    openapiLines.push('    ' + name + ':');
    openapiLines.push('      $ref: ./modular/components/responses/' + name + '.yaml');
  }
  openapiLines.push('  schemas:');
  for (const name of Object.keys(schemas)) {
    openapiLines.push('    ' + name + ':');
    openapiLines.push('      $ref: ./modular/components/schemas/' + name + '.yaml');
  }
  openapiLines.push('');
  openapiLines.push('paths:');
  for (const [pathName, methods] of Object.entries(paths)) {
    openapiLines.push('  ' + pathName + ':');
    openapiLines.push('    $ref: ./modular/paths/' + sanitizePathName(pathName) + '.yaml');
  }
  
  fs.writeFileSync(OPENAPI_FILE, openapiLines.join('\n'));
  console.log('Main openapi.yaml created');
  
  const pathCount = Object.keys(paths).length;
  const schemaCount = Object.keys(schemas).length;
  
  console.log(`\nModularization complete!\nCreated ${pathCount} path files and ${schemaCount} schema files\n`);
}

modularizeOpenAPI();