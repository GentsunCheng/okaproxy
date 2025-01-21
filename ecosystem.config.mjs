export const apps = [{
  name: "okaproxy",
  script: "./index.js",
  interpreter: 'bun',
  watch: true,
  instances: 'max',
  exec_mode: 'cluster',
}];
