require('esbuild').build({
  entryPoints: ['app.ts', 'app.css'],
  bundle: true,
  sourcemap: true,
  minify: true,
  outdir: 'dist',
}).catch(() => process.exit(1));
