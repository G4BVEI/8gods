<script>
	import { onMount, onDestroy } from 'svelte';
	import * as THREE from 'three';

	export let starCount = 400;
	export let nebulaCount = 2;
	export let speed = 6;
	export let targetFPS = 60;
	export let turbulence = 0;
	export let turbulenceFreq = 0.8;
	export let turbulenceGust = 0.3;

	let canvas;
	let renderer, scene, camera;
	let animationFrameId;

	onMount(() => {
		renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
		renderer.setPixelRatio(window.devicePixelRatio || 1);
		camera = new THREE.OrthographicCamera(0, 1, 0, 1, -1, 1);
		scene = new THREE.Scene();

		let W, H, cx, cy;
		function resize() {
			W = window.innerWidth;
			H = window.innerHeight;
			cx = W / 2;
			cy = H / 2;
			renderer.setSize(W, H);
			camera.right = W;
			camera.bottom = H;
			camera.updateProjectionMatrix();
		}
		resize();
		window.addEventListener('resize', resize);

		let isVisible = !document.hidden;
		const onVisibility = () => (isVisible = !document.hidden);
		document.addEventListener('visibilitychange', onVisibility);

		// ── Noise ─────────────────────────────────────────────────
		function noise1(x, s = 0) {
			return (
				Math.sin(x * 1.0 + s) * 0.5 +
				Math.sin(x * 2.31 + s * 1.73) * 0.25 +
				Math.sin(x * 5.17 + s * 3.13) * 0.125 +
				Math.sin(x * 11.7 + s * 5.37) * 0.063 +
				Math.sin(x * 23.1 + s * 11.1) * 0.031
			);
		}
		function noise2(x, y, s = 0) {
			return noise1(x + noise1(y + s, s * 2.1), s);
		}

		// ── FBM — fractal brownian motion ────────────────────────
		// More realistic than stacked sines: each octave adds detail
		function fbm(x, y, seed, octaves = 5) {
			let val = 0,
				amp = 0.5,
				freq = 1.0,
				max = 0;
			for (let o = 0; o < octaves; o++) {
				val += noise2(x * freq, y * freq, seed + o * 3.7) * amp;
				max += amp;
				amp *= 0.5;
				freq *= 2.1;
			}
			return val / max; // [-1, 1]
		}

		// ── Palette interpolation — density → RGBA ───────────────
		// This is how real nebula images work: map a scalar field to colour
		function samplePalette(t, stops) {
			t = Math.max(0, Math.min(1, t));
			for (let i = 0; i < stops.length - 1; i++) {
				if (t <= stops[i + 1][0]) {
					const u = (t - stops[i][0]) / (stops[i + 1][0] - stops[i][0]);
					const a = stops[i],
						b = stops[i + 1];
					return [
						a[1] + (b[1] - a[1]) * u,
						a[2] + (b[2] - a[2]) * u,
						a[3] + (b[3] - a[3]) * u,
						a[4] + (b[4] - a[4]) * u
					];
				}
			}
			return stops[stops.length - 1].slice(1);
		}

		// Nebula colour families — [t, r, g, b, a]
		// t=0 is vacuum/transparent, t=1 is hot dense core
		const PALETTES = {
			// Ionised oxygen — teal/cyan emission
			oiii: [
				[0.0, 0, 0, 0, 0],
				[0.15, 0, 20, 40, 15],
				[0.3, 0, 80, 140, 80],
				[0.5, 10, 170, 210, 160],
				[0.7, 40, 220, 240, 210],
				[0.85, 140, 245, 255, 235],
				[1.0, 220, 255, 255, 255]
			],
			// Hydrogen alpha — red/magenta emission
			halpha: [
				[0.0, 0, 0, 0, 0],
				[0.15, 40, 5, 20, 15],
				[0.3, 130, 10, 60, 80],
				[0.5, 200, 30, 100, 155],
				[0.7, 230, 60, 160, 205],
				[0.85, 250, 130, 210, 235],
				[1.0, 255, 200, 240, 255]
			],
			// Sulphur — warm orange/red
			sii: [
				[0.0, 0, 0, 0, 0],
				[0.15, 40, 10, 0, 15],
				[0.3, 140, 40, 0, 80],
				[0.5, 210, 90, 10, 155],
				[0.7, 240, 150, 40, 205],
				[0.85, 255, 200, 100, 235],
				[1.0, 255, 240, 180, 255]
			],
			// Reflection nebula — cool blue/violet
			reflect: [
				[0.0, 0, 0, 0, 0],
				[0.15, 5, 5, 40, 15],
				[0.3, 30, 20, 130, 80],
				[0.5, 60, 50, 200, 155],
				[0.7, 90, 80, 230, 205],
				[0.85, 150, 130, 250, 235],
				[1.0, 200, 180, 255, 255]
			]
		};

		const PALETTE_KEYS = Object.keys(PALETTES);

		// ── Texture generation ────────────────────────────────────
		// Three distinct layer types — each drawn differently
		function makeWispTex(paletteKey, seed) {
			// Wispy outer shell — sparse, filamentary, large reach
			const S = 512,
				c = document.createElement('canvas');
			c.width = c.height = S;
			const ctx = c.getContext('2d');
			const pal = PALETTES[paletteKey];
			const img = ctx.createImageData(S, S);
			const d = img.data;

			for (let py = 0; py < S; py++) {
				for (let px = 0; px < S; px++) {
					const nx = (px / S) * 3.0 - 1.5;
					const ny = (py / S) * 3.0 - 1.5;
					const r = Math.sqrt(nx * nx + ny * ny);

					// Heavy domain warp → long wispy tendrils
					const wx = nx + noise2(ny * 1.4 + seed, nx * 0.8 + seed * 2) * 1.2;
					const wy = ny + noise2(nx * 1.2 + seed * 3, ny * 1.6 + seed) * 1.2;

					const n = fbm(wx, wy, seed, 5) * 0.5 + 0.5;

					// Radial falloff — wisps only on the outer half
					const radial = Math.max(0, 1 - r * 0.9);
					const falloff = Math.pow(radial, 0.6);
					const density = (Math.max(0, n - 0.35) / 0.65) * falloff;

					const [ri, gi, bi, ai] = samplePalette(Math.pow(density, 1.4), pal);
					const idx = (py * S + px) * 4;
					d[idx] = ri;
					d[idx + 1] = gi;
					d[idx + 2] = bi;
					d[idx + 3] = Math.round(ai * 0.7); // wisps are transparent
				}
			}
			ctx.putImageData(img, 0, 0);
			return new THREE.CanvasTexture(c);
		}

		function makeBodyTex(paletteKey, seed) {
			// Main cloud body — medium density, two-zone structure
			const S = 512,
				c = document.createElement('canvas');
			c.width = c.height = S;
			const ctx = c.getContext('2d');
			const pal = PALETTES[paletteKey];
			const img = ctx.createImageData(S, S);
			const d = img.data;

			for (let py = 0; py < S; py++) {
				for (let px = 0; px < S; px++) {
					const nx = (px / S) * 2.5 - 1.25;
					const ny = (py / S) * 2.5 - 1.25;
					const r = Math.sqrt(nx * nx + ny * ny);

					// Moderate warp — lumpy but coherent
					const wx = nx + noise2(ny * 1.1 + seed, seed * 1.3) * 0.6;
					const wy = ny + noise2(nx * 0.9 + seed * 2.2, seed * 0.8) * 0.6;

					const n = fbm(wx * 1.3, wy * 1.3, seed, 5) * 0.5 + 0.5;

					const radial = Math.max(0, 1 - r * 1.1);
					const falloff = Math.pow(radial, 0.8);
					const density = (Math.max(0, n - 0.2) / 0.8) * falloff;

					const [ri, gi, bi, ai] = samplePalette(Math.pow(density, 1.1), pal);
					const idx = (py * S + px) * 4;
					d[idx] = ri;
					d[idx + 1] = gi;
					d[idx + 2] = bi;
					d[idx + 3] = Math.round(ai * 0.9);
				}
			}
			ctx.putImageData(img, 0, 0);
			return new THREE.CanvasTexture(c);
		}

		function makeCoreTex(paletteKey, seed) {
			// Bright dense core — hot glowing interior
			// Mix palette colour with a hot white for the innermost region
			const S = 256,
				c = document.createElement('canvas');
			c.width = c.height = S;
			const ctx = c.getContext('2d');
			const pal = PALETTES[paletteKey];
			const img = ctx.createImageData(S, S);
			const d = img.data;

			for (let py = 0; py < S; py++) {
				for (let px = 0; px < S; px++) {
					const nx = (px / S) * 2.0 - 1.0;
					const ny = (py / S) * 2.0 - 1.0;
					const r = Math.sqrt(nx * nx + ny * ny);

					// Light warp — core stays coherent
					const wx = nx + noise2(ny * 1.3 + seed, seed * 2) * 0.25;
					const wy = ny + noise2(nx * 1.1 + seed, seed * 3) * 0.25;

					const n = fbm(wx * 2.0, wy * 2.0, seed, 4) * 0.5 + 0.5;

					const radial = Math.max(0, 1 - r * 1.6);
					const falloff = Math.pow(radial, 1.2);
					const density = (Math.max(0, n - 0.1) / 0.9) * falloff;

					const [ri, gi, bi, ai] = samplePalette(Math.pow(density, 0.85), pal);

					// Hot-white blending at high density — stars heating the gas
					const heat = Math.pow(Math.max(0, density - 0.5) / 0.5, 2.0);
					const idx = (py * S + px) * 4;
					d[idx] = Math.round(ri + (255 - ri) * heat * 0.6);
					d[idx + 1] = Math.round(gi + (255 - gi) * heat * 0.5);
					d[idx + 2] = Math.round(bi + (255 - bi) * heat * 0.4);
					d[idx + 3] = Math.round(ai);
				}
			}
			ctx.putImageData(img, 0, 0);
			return new THREE.CanvasTexture(c);
		}

		// Build texture pools — one set per palette, generated once
		const texPools = {};
		for (const key of PALETTE_KEYS) {
			texPools[key] = {
				wisp: [makeWispTex(key, Math.random() * 100), makeWispTex(key, Math.random() * 100)],
				body: [makeBodyTex(key, Math.random() * 100), makeBodyTex(key, Math.random() * 100)],
				core: [makeCoreTex(key, Math.random() * 100)]
			};
		}

		function randTex(pool, type) {
			const arr = pool[type];
			return arr[Math.floor(Math.random() * arr.length)];
		}

		// ── Nebula groups ─────────────────────────────────────────
		// Each group = 3 meshes at different z offsets.
		// Wisp is deepest (furthest back) → you see it first as a faint glow.
		// Then body materialises. Then core rushes through.
		// Natural parallax since each layer projects from its own z.
		const LAYER_DEFS = [
			{ type: 'wisp', zOff: 500, sizeScale: 3.0, opScale: 0.55, spinMul: 0.6 },
			{ type: 'body', zOff: 200, sizeScale: 1.8, opScale: 0.85, spinMul: 1.0 },
			{ type: 'core', zOff: 0, sizeScale: 0.7, opScale: 1.0, spinMul: 1.4 }
		];

		function makeMesh(tex) {
			const mat = new THREE.MeshBasicMaterial({
				map: tex,
				transparent: true,
				opacity: 1,
				blending: THREE.AdditiveBlending,
				depthWrite: false,
				side: THREE.DoubleSide
			});
			const mesh = new THREE.Mesh(new THREE.PlaneGeometry(1, 1), mat);
			scene.add(mesh);
			return mesh;
		}

		function resetGroup(ng) {
			ng.z = W * 2 + Math.random() * W;
			ng.nx = (Math.random() - 0.5) * W * 1.8;
			ng.ny = (Math.random() - 0.5) * H * 1.8;
			ng.baseSize = W * (2.2 + Math.random() * 1.4);
			ng.angle = Math.random() * Math.PI * 2;
			ng.stretch = 0.5 + Math.random() * 0.6;
			ng.spin = (Math.random() - 0.5) * 0.003;
			ng.breathAmp = 0.03 + Math.random() * 0.04;
			ng.breathOff = Math.random() * Math.PI * 2;

			// Pick a random palette; slight chance of mixing two (cross-colour nebulae)
			const palKey = PALETTE_KEYS[Math.floor(Math.random() * PALETTE_KEYS.length)];
			const pool = texPools[palKey];

			ng.layers.forEach((layer, i) => {
				const def = LAYER_DEFS[i];
				layer.mesh.material.map = randTex(pool, def.type);
				layer.mesh.material.needsUpdate = true;
			});
		}

		const groups = [];
		for (let i = 0; i < nebulaCount; i++) {
			const layers = LAYER_DEFS.map((def) => ({
				def,
				mesh: makeMesh(texPools[PALETTE_KEYS[0]][def.type][0])
			}));
			const ng = {
				layers,
				z: 0,
				nx: 0,
				ny: 0,
				baseSize: 0,
				angle: 0,
				stretch: 1,
				spin: 0,
				breathAmp: 0,
				breathOff: 0
			};
			resetGroup(ng);
			ng.z = Math.random() * W * 2;
			groups.push(ng);
		}

		// ── Stars ──────────────────────────────────────────────────
		const starData = new Float32Array(starCount * 3);
		const positions = new Float32Array(starCount * 6);
		const colors = new Float32Array(starCount * 6);

		for (let i = 0; i < starCount; i++) {
			starData[i * 3] = (Math.random() - 0.5) * 2.5;
			starData[i * 3 + 1] = (Math.random() - 0.5) * 2.5;
			starData[i * 3 + 2] = Math.random();
		}

		const starGeo = new THREE.BufferGeometry();
		starGeo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
		starGeo.setAttribute('color', new THREE.BufferAttribute(colors, 3));
		scene.add(new THREE.LineSegments(starGeo, new THREE.LineBasicMaterial({ vertexColors: true })));

		// ── Helpers ────────────────────────────────────────────────
		function getOffset(z, t) {
			return {
				x: Math.sin(z * 0.0006 + t * 0.5) * 280,
				y: Math.cos(z * 0.0008 + t * 0.4) * 200
			};
		}

		function getTurbulence(t) {
			if (turbulence <= 0) return { x: 0, y: 0, roll: 0 };
			const amp = turbulence * W * 0.04;
			const jx = noise1(t * turbulenceFreq * 4.1, 0) * amp * 0.4;
			const jy = noise1(t * turbulenceFreq * 3.7, 5.3) * amp * 0.4;
			const gx = noise1(t * turbulenceFreq * turbulenceGust, 2.1) * amp;
			const gy = noise1(t * turbulenceFreq * turbulenceGust * 0.9, 8.4) * amp;
			const roll = noise1(t * turbulenceFreq * 0.6, 1.7) * turbulence * 0.025;
			return { x: jx + gx, y: jy + gy, roll };
		}

		// ── Loop ───────────────────────────────────────────────────
		const frameInterval = 1000 / targetFPS;
		let lastTime = 0,
			time = 0;

		function draw(now) {
			animationFrameId = requestAnimationFrame(draw);
			if (!isVisible) return;

			const dt = now - lastTime;
			if (dt < frameInterval) return;
			lastTime = now - (dt % frameInterval);
			time += 0.012;

			renderer.setClearColor(0x000308, 1);

			const turb = getTurbulence(time);
			scene.rotation.z = turb.roll;
			scene.position.set(turb.x, turb.y, 0);

			// — Nebula groups —
			const hor = W * 2;
			for (const ng of groups) {
				ng.z -= speed * 0.7;
				ng.angle += ng.spin;
				if (ng.z <= -800) resetGroup(ng);

				const breathe = 1 + Math.sin(time * 0.7 + ng.breathOff) * ng.breathAmp;

				for (const layer of ng.layers) {
					const { def, mesh } = layer;
					const lz = ng.z + def.zOff; // each layer projects from its own z

					// Fade based on this layer's z — wisp visible from far, core only up close
					let op = 0;
					if (lz > 0 && lz < hor) {
						const fadeIn = Math.min(1, Math.pow((hor - lz) / hor, 2.0) * 5.0);
						const fadeOut = Math.max(0, Math.min(1, (lz - 50) / 600));
						op = def.opScale * fadeIn * fadeOut * breathe;
					}

					mesh.visible = op > 0;
					if (op > 0) {
						const o = getOffset(lz, time);
						const sc = W / Math.max(1, lz);
						const sz = ng.baseSize * sc * def.sizeScale;
						mesh.position.set((ng.nx + o.x) * (sc * 0.1) + cx, (ng.ny + o.y) * (sc * 0.1) + cy, 0);
						mesh.rotation.z = ng.angle * def.spinMul;
						mesh.scale.set(sz, sz * ng.stretch, 1);
						mesh.material.opacity = op;
					}
				}
			}

			// — Stars —
			for (let i = 0; i < starCount; i++) {
				starData[i * 3 + 2] -= speed / W;
				if (starData[i * 3 + 2] <= 0) starData[i * 3 + 2] = 1;

				const rx = starData[i * 3] * W;
				const ry = starData[i * 3 + 1] * H;
				const z = starData[i * 3 + 2] * W;

				const o1 = getOffset(z, time),
					s1 = W / z;
				const o2 = getOffset(z + 25, time),
					s2 = W / (z + 25);

				const hx = (rx + o1.x) * (s1 * 0.1) + cx;
				const hy = (ry + o1.y) * (s1 * 0.1) + cy;
				const tx = (rx + o2.x) * (s2 * 0.1) + cx;
				const ty = (ry + o2.y) * (s2 * 0.1) + cy;

				const proximity = 1 - z / W;
				const alpha = Math.min(1, Math.pow(proximity, 2.2) * 2.5);
				const r = 0.239 * alpha,
					g = 0.949 * alpha,
					b = 0.878 * alpha;
				const v = i * 6;

				positions[v] = tx;
				positions[v + 1] = ty;
				positions[v + 2] = 0;
				positions[v + 3] = hx;
				positions[v + 4] = hy;
				positions[v + 5] = 0;
				colors[v] = r * 0.15;
				colors[v + 1] = g * 0.15;
				colors[v + 2] = b * 0.15;
				colors[v + 3] = r;
				colors[v + 4] = g;
				colors[v + 5] = b;
			}

			starGeo.attributes.position.needsUpdate = true;
			starGeo.attributes.color.needsUpdate = true;

			renderer.render(scene, camera);
		}

		requestAnimationFrame(draw);

		return () => {
			window.removeEventListener('resize', resize);
			document.removeEventListener('visibilitychange', onVisibility);
			if (animationFrameId) cancelAnimationFrame(animationFrameId);
			renderer.dispose();
			for (const key of PALETTE_KEYS) {
				const p = texPools[key];
				[...p.wisp, ...p.body, ...p.core].forEach((t) => t.dispose());
			}
			starGeo.dispose();
		};
	});

	onDestroy(() => {
		if (animationFrameId) cancelAnimationFrame(animationFrameId);
	});
</script>

<canvas
	bind:this={canvas}
	class="pointer-events-none fixed top-0 left-0 -z-1 h-full w-full bg-[#000308]"
></canvas>
