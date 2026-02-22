<script>
	import { onMount } from 'svelte';
	import CyanNebula from './shaders/CyanNebula.frag?raw';
	import Nebula from './shaders/Nebula.frag?raw';

	// Using Svelte 5 $state
	let Mode = $props();
	let shaderSource = Mode === 'Cataclysm' ? Nebula : CyanNebula;
	let lastTime = 0;
	const targetFPS = 60; // Set your desired FPS here
	const fpsInterval = 1000 / targetFPS;

	let canvas;
	let gl;
	let program;
	let animationFrame;
	let resizeObserver;

	const vertexShaderSource = `
        attribute vec2 position;
        void main() { gl_Position = vec4(position, 0.0, 1.0); }
    `;

	onMount(() => {
		gl = canvas.getContext('webgl');
		if (!gl) return;

		const createShader = (gl, type, source) => {
			const s = gl.createShader(type);
			gl.shaderSource(s, source);
			gl.compileShader(s);
			if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
				console.error(gl.getShaderInfoLog(s));
			}
			return s;
		};

		const vs = createShader(gl, gl.VERTEX_SHADER, vertexShaderSource);
		const fs = createShader(gl, gl.FRAGMENT_SHADER, shaderSource);

		program = gl.createProgram();
		gl.attachShader(program, vs);
		gl.attachShader(program, fs);
		gl.linkProgram(program);
		gl.useProgram(program);

		const buffer = gl.createBuffer();
		gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
		gl.bufferData(
			gl.ARRAY_BUFFER,
			new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]),
			gl.STATIC_DRAW
		);

		const positionLoc = gl.getAttribLocation(program, 'position');
		gl.enableVertexAttribArray(positionLoc);
		gl.vertexAttribPointer(positionLoc, 2, gl.FLOAT, false, 0, 0);

		const iTimeLoc = gl.getUniformLocation(program, 'iTime');
		const iResLoc = gl.getUniformLocation(program, 'iResolution');

		// Robust resizing logic
		resizeObserver = new ResizeObserver((entries) => {
			for (let entry of entries) {
				const { width, height } = entry.contentRect;
				canvas.width = width;
				canvas.height = height;
				gl.viewport(0, 0, width, height);
				gl.uniform3f(iResLoc, width, height, 1.0);
			}
		});
		resizeObserver.observe(canvas);

		const render = (time) => {
			animationFrame = requestAnimationFrame(render);

			// Calculate how much time passed since the last frame
			const deltaTime = time - lastTime;

			// If enough time has passed, render the frame
			if (deltaTime >= fpsInterval) {
				// Adjust lastTime, but account for the "remainder" to keep timing smooth
				lastTime = time - (deltaTime % fpsInterval);

				// 1. Update Uniforms
				gl.uniform1f(iTimeLoc, time * 0.001);

				// 2. Draw
				gl.drawArrays(gl.TRIANGLES, 0, 6);
			}
		};

		animationFrame = requestAnimationFrame(render);

		return () => {
			cancelAnimationFrame(animationFrame);
			resizeObserver.disconnect();
			gl.deleteProgram(program);
		};
	});
</script>

<canvas bind:this={canvas} class="block h-full w-full bg-black"></canvas>
