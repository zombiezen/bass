const { stimulusPlugin } = require('esbuild-plugin-stimulus');

require('esbuild').build({
  entryPoints: ['app.ts', 'app.css'],
  bundle: true,
  sourcemap: true,
  minify: true,
  outdir: 'dist',
  plugins: [stimulusPlugin()],
}).catch(() => process.exit(1));
