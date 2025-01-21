module.exports = {
    apps: [
      {
        name: 'okaproxy',
        script: 'bun',
        args: 'index.js',
        watch: true,
        instances: 'max',
        exec_mode: 'cluster',
        env: {
          NODE_ENV: 'production',
        },
      },
    ],
  };
  