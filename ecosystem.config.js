module.exports = {
    apps: [
      {
        name: 'okaproxy',
        script: 'index.js',
        interpreter: 'bun',
        watch: true,
        instances: 'max',
        exec_mode: 'cluster',
        env: {
          NODE_ENV: 'production',
        },
      },
    ],
  };
  