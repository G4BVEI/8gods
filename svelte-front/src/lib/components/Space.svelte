<script>
	import { onMount, onDestroy } from 'svelte';
	import * as THREE from 'three';

	export let starCount = 400;
	export let fogCount = 15; // more smaller patches vs fewer big nebulae
	export let speed = 10;
	export let targetFPS = 60;
	export let turbulence = 2;
	export let turbulenceFreq = 0.8;
	export let turbulenceGust = 0.3;
	export let slicesPerFog = 8; // fewer slices — these are small dense pockets
	export let fogDensity = 0.22; // higher per-slice opacity since they're tight
	export let fogSizeMin = 0.3; // fraction of W
	export let fogSizeMax = 0.9;

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

		// ── Noise ──────────────────────────────────────────────────
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
		function fbm(x, y, seed, oct = 5) {
			let v = 0,
				a = 0.5,
				f = 1.0,
				m = 0;
			for (let o = 0; o < oct; o++) {
				v += noise2(x * f, y * f, seed + o * 3.7) * a;
				m += a;
				a *= 0.5;
				f *= 2.1;
			}
			return v / m;
		}

		// ── Arcane palettes ────────────────────────────────────────
		// Space-arcane: unnatural glows, cursed hues, ether energy.
		// Each is [t, r, g, b, a] — density→colour mapping.
		const PALETTES = {
			// Void purple — deep arcane energy, like raw magic in a vacuum
			void: [
				[0.0, 0, 0, 0, 0],
				[0.12, 15, 0, 30, 12],
				[0.28, 60, 5, 110, 80],
				[0.5, 130, 20, 220, 175],
				[0.72, 190, 70, 255, 225],
				[0.88, 235, 160, 255, 248],
				[1.0, 255, 230, 255, 255]
			],
			// Ether green — sickly arcane residue, corrupted space
			ether: [
				[0.0, 0, 0, 0, 0],
				[0.12, 0, 25, 10, 12],
				[0.28, 10, 100, 30, 80],
				[0.5, 30, 200, 80, 175],
				[0.72, 80, 240, 130, 225],
				[0.88, 180, 255, 200, 248],
				[1.0, 230, 255, 240, 255]
			],
			// Blood rift — crimson tears in space, dangerous areas
			rift: [
				[0.0, 0, 0, 0, 0],
				[0.12, 30, 2, 2, 12],
				[0.28, 120, 8, 10, 80],
				[0.5, 210, 25, 30, 175],
				[0.72, 245, 70, 55, 225],
				[0.88, 255, 160, 120, 248],
				[1.0, 255, 230, 200, 255]
			],
			// Frost sigil — cold blue-white arcane runes, frozen magic
			frost: [
				[0.0, 0, 0, 0, 0],
				[0.12, 5, 15, 40, 12],
				[0.28, 20, 60, 160, 80],
				[0.5, 50, 130, 235, 175],
				[0.72, 110, 195, 255, 225],
				[0.88, 200, 235, 255, 248],
				[1.0, 240, 250, 255, 255]
			],
			// Gold aura — ancient relics, trapped soul energy
			soul: [
				[0.0, 0, 0, 0, 0],
				[0.12, 30, 18, 0, 12],
				[0.28, 120, 75, 5, 80],
				[0.5, 220, 160, 20, 175],
				[0.72, 255, 220, 70, 225],
				[0.88, 255, 245, 180, 248],
				[1.0, 255, 255, 235, 255]
			],
			// Null — almost invisible grey static, background arcane noise
			null: [
				[0.0, 0, 0, 0, 0],
				[0.15, 10, 10, 15, 10],
				[0.35, 35, 35, 50, 60],
				[0.6, 80, 80, 110, 140],
				[0.8, 140, 140, 180, 210],
				[1.0, 200, 200, 230, 255]
			]
		};
		const PAL_KEYS = Object.keys(PALETTES);

		function samplePalette(t, pal) {
			t = Math.max(0, Math.min(1, t));
			for (let i = 0; i < pal.length - 1; i++) {
				if (t <= pal[i + 1][0]) {
					const u = (t - pal[i][0]) / (pal[i + 1][0] - pal[i][0]);
					return pal[i].slice(1).map((v, j) => v + (pal[i + 1][j + 1] - v) * u);
				}
			}
			return pal[pal.length - 1].slice(1);
		}

		// ── Fog tile textures ──────────────────────────────────────
		// Smaller patches want rounder, denser, cloudier shapes.
		// Three variants: puff (round dense blob), wisp (tendrilly), ember (bright hot core)
		function makePuffTile(palKey, seed) {
			const S = 256,
				c = document.createElement('canvas');
			c.width = c.height = S;
			const ctx = c.getContext('2d');
			const pal = PALETTES[palKey];
			const img = ctx.createImageData(S, S);
			const d = img.data;
			for (let py = 0; py < S; py++) {
				for (let px = 0; px < S; px++) {
					const nx = (px / S) * 2.2 - 1.1;
					const ny = (py / S) * 2.2 - 1.1;
					const r = Math.sqrt(nx * nx + ny * ny);

					// Mild warp — keep puffs coherent and round-ish
					const wx = nx + noise2(ny * 1.2 + seed, seed * 1.4) * 0.4;
					const wy = ny + noise2(nx * 1.0 + seed * 2.1, seed * 0.9) * 0.4;

					const n = fbm(wx * 1.4, wy * 1.4, seed, 4) * 0.5 + 0.5;

					// Tight radial falloff — these are compact blobs
					const radial = Math.max(0, 1 - r * 1.15);
					const falloff = Math.pow(radial, 0.65);
					const density = (Math.max(0, n - 0.18) / 0.82) * falloff;

					const [ri, gi, bi, ai] = samplePalette(Math.pow(density, 1.05), pal);
					const heat = Math.pow(Math.max(0, density - 0.55) / 0.45, 2.2);
					const idx = (py * S + px) * 4;
					d[idx] = Math.min(255, Math.round(ri + (255 - ri) * heat * 0.5));
					d[idx + 1] = Math.min(255, Math.round(gi + (255 - gi) * heat * 0.35));
					d[idx + 2] = Math.min(255, Math.round(bi + (255 - bi) * heat * 0.25));
					d[idx + 3] = Math.round(ai);
				}
			}
			ctx.putImageData(img, 0, 0);
			return new THREE.CanvasTexture(c);
		}

		function makeWispTile(palKey, seed) {
			const S = 256,
				c = document.createElement('canvas');
			c.width = c.height = S;
			const ctx = c.getContext('2d');
			const pal = PALETTES[palKey];
			const img = ctx.createImageData(S, S);
			const d = img.data;
			for (let py = 0; py < S; py++) {
				for (let px = 0; px < S; px++) {
					const nx = (px / S) * 2.8 - 1.4;
					const ny = (py / S) * 2.8 - 1.4;
					const r = Math.sqrt(nx * nx + ny * ny);

					// Heavy warp — wisps are tendrilly and organic
					const w1x = nx + noise2(ny * 1.5 + seed, seed * 1.6) * 0.9;
					const w1y = ny + noise2(nx * 1.3 + seed * 2.4, seed * 0.7) * 0.9;
					const w2x = w1x + noise2(w1y * 2.6 + seed * 3.2, seed * 1.1) * 0.35;
					const w2y = w1y + noise2(w1x * 2.8 + seed * 4.5, seed * 2.3) * 0.35;

					const n = fbm(w2x, w2y, seed, 5) * 0.5 + 0.5;

					const radial = Math.max(0, 1 - r * 0.92);
					const falloff = Math.pow(radial, 0.45);
					const density = (Math.max(0, n - 0.3) / 0.7) * falloff;

					const [ri, gi, bi, ai] = samplePalette(Math.pow(density, 1.3), pal);
					const idx = (py * S + px) * 4;
					d[idx] = ri;
					d[idx + 1] = gi;
					d[idx + 2] = bi;
					d[idx + 3] = Math.round(ai * 0.65); // wisps are inherently faint
				}
			}
			ctx.putImageData(img, 0, 0);
			return new THREE.CanvasTexture(c);
		}

		function makeEmberTile(palKey, seed) {
			// Tight bright core — the arcane hot spot
			const S = 128,
				c = document.createElement('canvas');
			c.width = c.height = S;
			const ctx = c.getContext('2d');
			const pal = PALETTES[palKey];
			const img = ctx.createImageData(S, S);
			const d = img.data;
			for (let py = 0; py < S; py++) {
				for (let px = 0; px < S; px++) {
					const nx = (px / S) * 2.0 - 1.0;
					const ny = (py / S) * 2.0 - 1.0;
					const r = Math.sqrt(nx * nx + ny * ny);

					const wx = nx + noise2(ny * 1.1 + seed, seed * 2) * 0.18;
					const wy = ny + noise2(nx * 0.9 + seed, seed * 3) * 0.18;
					const n = fbm(wx * 2.5, wy * 2.5, seed, 3) * 0.5 + 0.5;

					const radial = Math.max(0, 1 - r * 1.9);
					const falloff = Math.pow(radial, 1.4);
					const density = (Math.max(0, n - 0.08) / 0.92) * falloff;

					const [ri, gi, bi, ai] = samplePalette(Math.pow(density, 0.8), pal);
					const heat = Math.pow(Math.max(0, density - 0.4) / 0.6, 1.8);
					const idx = (py * S + px) * 4;
					d[idx] = Math.min(255, Math.round(ri + (255 - ri) * heat * 0.7));
					d[idx + 1] = Math.min(255, Math.round(gi + (255 - gi) * heat * 0.55));
					d[idx + 2] = Math.min(255, Math.round(bi + (255 - bi) * heat * 0.4));
					d[idx + 3] = Math.round(ai);
				}
			}
			ctx.putImageData(img, 0, 0);
			return new THREE.CanvasTexture(c);
		}

		// Build pools
		const POOL = {};
		for (const key of PAL_KEYS) {
			POOL[key] = {
				puff: Array.from({ length: 3 }, (_, i) => makePuffTile(key, i * 6.3 + 1)),
				wisp: Array.from({ length: 3 }, (_, i) => makeWispTile(key, i * 9.1 + 20)),
				ember: Array.from({ length: 2 }, (_, i) => makeEmberTile(key, i * 13.7 + 50))
			};
		}

		function pickTile(pool, type) {
			const arr = pool[type];
			return arr[Math.floor(Math.random() * arr.length)];
		}

		// Slice tier by stack position — puffs in body, wisps at edges, ember at centre
		function sliceTier(t) {
			const dist = Math.abs(t - 0.5) * 2;
			if (dist < 0.25) return 'ember';
			if (dist < 0.65) return 'puff';
			return 'wisp';
		}
		function sliceBaseOp(t) {
			const dist = Math.abs(t - 0.5) * 2;
			return Math.pow(1 - dist, 1.2);
		}

		// ── Fog volumes ────────────────────────────────────────────
		function buildMesh(tex) {
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

		function resetFog(fog) {
			const baseSize = W * (fogSizeMin + Math.random() * (fogSizeMax - fogSizeMin));
			fog.z = W * 2 + Math.random() * W * 1.5;
			fog.nx = (Math.random() - 0.5) * W * 2.2;
			fog.ny = (Math.random() - 0.5) * H * 2.2;
			fog.baseSize = baseSize;
			fog.angle = Math.random() * Math.PI * 2;
			fog.stretch = 0.6 + Math.random() * 0.65;
			fog.spin = (Math.random() - 0.5) * 0.006;

			// Lateral drift — arcane fog drifts sideways in space, not just at you
			fog.driftX = (Math.random() - 0.5) * speed * 0.35;
			fog.driftY = (Math.random() - 0.5) * speed * 0.2;

			// Flicker — arcane energy is unstable
			fog.flickerAmp = 0.05 + Math.random() * 0.18;
			fog.flickerFreq = 1.5 + Math.random() * 4.0;
			fog.flickerOff = Math.random() * Math.PI * 2;

			// Breathe — slow pulsing on top of flicker
			fog.breathAmp = 0.04 + Math.random() * 0.06;
			fog.breathOff = Math.random() * Math.PI * 2;

			// Depth spread proportional to size — small puffs are shallower
			const depthSpread = baseSize * (1.8 + Math.random() * 1.2);

			const palKey = PAL_KEYS[Math.floor(Math.random() * PAL_KEYS.length)];
			const pool = POOL[palKey];

			fog.slices.forEach((sl, i) => {
				const t = i / Math.max(1, slicesPerFog - 1);
				sl.relZ =
					(1 - t) * depthSpread + (Math.random() - 0.5) * (depthSpread / slicesPerFog) * 0.7;
				sl.jx = (Math.random() - 0.5) * 0.12;
				sl.jy = (Math.random() - 0.5) * 0.12;
				sl.sizeMul = 0.8 + t * 0.5 + Math.random() * 0.12;
				sl.angleOff = (Math.random() - 0.5) * 0.9;
				sl.baseOp = sliceBaseOp(t) * fogDensity;
				sl.mesh.material.map = pickTile(pool, sliceTier(t));
				sl.mesh.material.needsUpdate = true;
			});
		}

		const fogs = [];
		const initTex = POOL[PAL_KEYS[0]].puff[0];

		for (let i = 0; i < fogCount; i++) {
			const slices = Array.from({ length: slicesPerFog }, () => ({
				mesh: buildMesh(initTex),
				relZ: 0,
				jx: 0,
				jy: 0,
				sizeMul: 1,
				angleOff: 0,
				baseOp: 0
			}));
			const fog = { slices };
			resetFog(fog);
			// Stagger initial z so they're distributed through the tunnel
			fog.z = Math.random() * W * 2;
			// Offset nx/ny so they start spread out too
			fogs.push(fog);
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

			const hor = W * 2;

			for (const fog of fogs) {
				fog.z -= speed * 0.7;
				fog.nx += fog.driftX * 0.016; // slow drift accumulates
				fog.ny += fog.driftY * 0.016;
				fog.angle += fog.spin;

				const maxRelZ = Math.max(...fog.slices.map((s) => s.relZ));
				if (fog.z + maxRelZ < -300) resetFog(fog);

				// Flicker: fast arcane instability + slow breathe
				const flicker =
					1 +
					Math.sin(time * fog.flickerFreq + fog.flickerOff) * fog.flickerAmp +
					Math.sin(time * fog.flickerFreq * 2.3 + fog.flickerOff * 1.7) * fog.flickerAmp * 0.4 +
					Math.sin(time * 0.65 + fog.breathOff) * fog.breathAmp;

				for (const sl of fog.slices) {
					const lz = fog.z + sl.relZ;

					let op = 0;
					if (lz > 0 && lz < hor) {
						const fadeIn = Math.min(1, Math.pow((hor - lz) / hor, 1.6) * 4.0);
						const fadeOut = Math.max(0, Math.min(1, lz / 400));
						op = sl.baseOp * fadeIn * fadeOut * flicker;
					}

					sl.mesh.visible = op > 0.003;
					if (sl.mesh.visible) {
						const o = getOffset(lz, time);
						const sc = W / Math.max(1, lz);
						const sz = fog.baseSize * sc * sl.sizeMul;
						const jx = sl.jx * fog.baseSize * sc;
						const jy = sl.jy * fog.baseSize * sc;

						sl.mesh.position.set(
							(fog.nx + o.x) * (sc * 0.1) + cx + jx,
							(fog.ny + o.y) * (sc * 0.1) + cy + jy,
							0
						);
						sl.mesh.rotation.z = fog.angle + sl.angleOff;
						sl.mesh.scale.set(sz, sz * fog.stretch, 1);
						sl.mesh.material.opacity = Math.max(0, Math.min(1, op));
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
			for (const key of PAL_KEYS) {
				const p = POOL[key];
				[...p.puff, ...p.wisp, ...p.ember].forEach((t) => t.dispose());
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
