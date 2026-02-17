<script>
	import { onMount, onDestroy } from 'svelte';

	// Props for customization
	export let starCount = 400;
	export let nebulaCount = 2;
	export let speed = 6;
	export let targetFPS = 60;

	let canvas;
	let ctx;
	let animationFrameId;
	let isVisible = true;

	onMount(() => {
		ctx = canvas.getContext('2d', { alpha: false });
		let w, h, cx, cy;
		let dpr = window.devicePixelRatio || 1;

		// Visibility change handler
		const handleVisibilityChange = () => {
			isVisible = !document.hidden;
		};
		document.addEventListener('visibilitychange', handleVisibilityChange);

		const createNebulaSprite = (color1, color2) => {
			const c = document.createElement('canvas');
			const spriteSize = 1024;
			c.width = spriteSize;
			c.height = spriteSize;
			const fctx = c.getContext('2d');
			const grad = fctx.createRadialGradient(
				spriteSize / 2,
				spriteSize / 2,
				0,
				spriteSize / 2,
				spriteSize / 2,
				spriteSize / 2
			);
			grad.addColorStop(0, color1);
			grad.addColorStop(0.4, color2);
			grad.addColorStop(1, 'rgba(0, 0, 0, 0)');
			fctx.fillStyle = grad;
			fctx.fillRect(0, 0, spriteSize, spriteSize);
			return c;
		};

		const cyanSprite = createNebulaSprite('rgba(0, 255, 240, 0.2)', 'rgba(0, 50, 100, 0.08)');
		const purpleSprite = createNebulaSprite('rgba(150, 50, 255, 0.15)', 'rgba(40, 0, 80, 0.05)');

		const resize = () => {
			const displayWidth = window.innerWidth;
			const displayHeight = window.innerHeight;
			dpr = window.devicePixelRatio || 1;

			canvas.width = displayWidth * dpr;
			canvas.height = displayHeight * dpr;
			canvas.style.width = `${displayWidth}px`;
			canvas.style.height = `${displayHeight}px`;

			ctx.scale(dpr, dpr);

			w = displayWidth;
			h = displayHeight;
			cx = w / 2;
			cy = h / 2;
		};

		window.addEventListener('resize', resize);
		resize();

		const horizon = w * 2;
		let time = 0;

		const stars = new Float32Array(starCount * 3);
		let fog = [];

		// Initialize stars
		for (let i = 0; i < starCount * 3; i += 3) {
			stars[i] = (Math.random() - 0.5) * w * 5;
			stars[i + 1] = (Math.random() - 0.5) * h * 5;
			stars[i + 2] = Math.random() * w;
		}

		const resetFog = (f) => {
			f.z = horizon + Math.random() * w;
			f.x = (Math.random() - 0.5) * w * 2.5;
			f.y = (Math.random() - 0.5) * h * 2.5;
			f.size = w * 2.2;
			f.angle = Math.random() * Math.PI * 2;
			f.type = Math.random() > 0.5 ? cyanSprite : purpleSprite;
			f.stretch = 0.4 + Math.random() * 0.6;
			f.spin = (Math.random() - 0.5) * 0.005;
		};

		// Initialize nebulae
		for (let i = 0; i < nebulaCount; i++) {
			let f = {};
			resetFog(f);
			f.z = Math.random() * horizon;
			fog.push(f);
		}

		const getOffset = (z, t) => {
			const x = Math.sin(z * 0.0006 + t * 0.5) * 280;
			const y = Math.cos(z * 0.0008 + t * 0.4) * 200;
			return { x, y };
		};

		// FPS limiting
		const frameInterval = 1000 / targetFPS;
		let lastFrameTime = 0;

		function draw(currentTime) {
			animationFrameId = requestAnimationFrame(draw);

			// Pause when tab is hidden
			if (!isVisible) return;

			// FPS limiting
			const deltaTime = currentTime - lastFrameTime;
			if (deltaTime < frameInterval) return;
			lastFrameTime = currentTime - (deltaTime % frameInterval);

			time += 0.012;

			// Clear with devicePixelRatio consideration
			ctx.fillStyle = '#000308';
			ctx.fillRect(0, 0, w, h);

			// Draw nebulae
			ctx.globalCompositeOperation = 'screen';
			for (let f of fog) {
				f.z -= speed * 0.7;
				f.angle += f.spin;

				if (f.z <= -200) resetFog(f);

				const offset = getOffset(f.z, time);
				const scale = w / Math.max(1, f.z);
				const x = (f.x + offset.x) * (scale * 0.1) + cx;
				const y = (f.y + offset.y) * (scale * 0.1) + cy;
				const sz = f.size * scale;

				// Smooth fade in/out
				let opacity = 0;
				if (f.z < horizon && f.z > 0) {
					const fadeIn = Math.min(1, (horizon - f.z) / 600);
					const fadeOut = Math.max(0, Math.min(1, (f.z - 50) / 400));
					opacity = 0.25 * fadeIn * fadeOut;
				}

				if (opacity > 0) {
					ctx.globalAlpha = opacity;
					ctx.save();
					ctx.translate(x, y);
					ctx.rotate(f.angle);
					ctx.scale(1.3, f.stretch);
					ctx.drawImage(f.type, -sz / 2, -sz / 2, sz, sz);
					ctx.restore();
				}
			}

			// Draw stars
			ctx.globalCompositeOperation = 'source-over';
			ctx.globalAlpha = 1.0;
			for (let i = 0; i < starCount * 3; i += 3) {
				stars[i + 2] -= speed;
				if (stars[i + 2] <= 0) stars[i + 2] = w;

				const z = stars[i + 2];
				const offset = getOffset(z, time);
				const scale = w / z;
				const sx = (stars[i] + offset.x) * (scale * 0.1) + cx;
				const sy = (stars[i + 1] + offset.y) * (scale * 0.1) + cy;

				const pz = z + 25;
				const pOffset = getOffset(pz, time);
				const pScale = w / pz;
				const ex = (stars[i] + pOffset.x) * (pScale * 0.1) + cx;
				const ey = (stars[i + 1] + pOffset.y) * (pScale * 0.1) + cy;

				const opacity = Math.min(1, (1 - z / w) * 1.5);
				if (sx > -100 && sx < w + 100) {
					ctx.lineWidth = opacity * 3;
					ctx.strokeStyle = `rgba(200, 250, 255, ${opacity * Math.min(1, z / 100)})`;
					ctx.beginPath();
					ctx.moveTo(ex, ey);
					ctx.lineTo(sx, sy);
					ctx.stroke();
				}
			}
		}

		draw(0);

		// Cleanup
		return () => {
			window.removeEventListener('resize', resize);
			document.removeEventListener('visibilitychange', handleVisibilityChange);
			if (animationFrameId) {
				cancelAnimationFrame(animationFrameId);
			}
		};
	});

	onDestroy(() => {
		if (animationFrameId) {
			cancelAnimationFrame(animationFrameId);
		}
	});
</script>

<canvas bind:this={canvas}></canvas>

<style>
	canvas {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background: #000;
		z-index: -1;
		pointer-events: none;
	}
</style>
