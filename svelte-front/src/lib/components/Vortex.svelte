<script>
	import { onMount } from 'svelte';
	import * as THREE from 'three';

	let canvas;
	let renderer, scene, camera, material;
	let frameId;

	// Ported from https://www.shadertoy.com/view/4d3SWM
	const fragmentShader = `
    uniform float iTime;
    uniform vec2 iResolution;

    void main() {
      // Correct for aspect ratio and center coordinates
      vec2 uv = (gl_FragCoord.xy / iResolution.xy) - 0.5;
      uv.x *= iResolution.x / iResolution.y;

      // The "Forward Movement" and rotation logic from the shader
      float t = iTime * .1 + ((.25 + .05 * sin(iTime * .1)) / (length(uv.xy) + .07)) * 2.2;
      float si = sin(t);
      float co = cos(t);
      mat2 ma = mat2(co, si, -si, co);

      float v1, v2, v3;
      v1 = v2 = v3 = 0.0;
      float s = 0.0;

      // Raymarching through the fractal nebula layers
      for (int i = 0; i < 90; i++) {
        vec3 p = s * vec3(uv, 0.0);
        p.xy *= ma;
        p += vec3(.22, .3, s - 1.5 - sin(iTime * .13) * .1);

        // The fractal "Fold" loop - this creates the wispy nebula detail
        for (int j = 0; j < 8; j++) {
          p = abs(p) / dot(p, p) - 0.659;
        }

        v1 += dot(p, p) * .0015 * (1.8 + sin(length(uv.xy * 13.0) + .5  * iTime));
        v2 += dot(p, p) * .0013 * (1.5 + sin(length(uv.xy * 14.5) + 1.2 * iTime));
        v3 += length(p.xy * 10.) * .0003;
        s += .035;
      }

      float len = length(uv);
      v1 *= smoothstep(.7, .0, len);
      v2 *= smoothstep(.5, .0, len);
      v3 *= smoothstep(.9, .0, len);

      // Color mixing (Blues, Cyans, Purples)
      vec3 col = vec3( v3 * (1.5 + sin(iTime * .2) * .4),
                      (v1 + v3) * .3,
                       v2)
                 + smoothstep(0.2, .0, len) * .85
                 + smoothstep(.0, .6, v3) * .3;

      gl_FragColor = vec4(min(pow(col, vec3(1.2)), 1.0), 1.0);
    }
  `;

	onMount(() => {
		scene = new THREE.Scene();
		// Orthographic camera because we are rendering a full-screen shader
		camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);

		renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
		renderer.setSize(window.innerWidth, window.innerHeight);

		const geometry = new THREE.PlaneGeometry(2, 2);
		material = new THREE.ShaderMaterial({
			fragmentShader,
			uniforms: {
				iTime: { value: 0 },
				iResolution: { value: new THREE.Vector2(window.innerWidth, window.innerHeight) }
			}
		});

		const mesh = new THREE.Mesh(geometry, material);
		scene.add(mesh);

		const clock = new THREE.Clock();

		const animate = () => {
			frameId = requestAnimationFrame(animate);
			material.uniforms.iTime.value = clock.getElapsedTime();
			renderer.render(scene, camera);
		};

		animate();

		const handleResize = () => {
			const w = window.innerWidth;
			const h = window.innerHeight;
			renderer.setSize(w, h);
			material.uniforms.iResolution.value.set(w, h);
		};

		window.addEventListener('resize', handleResize);

		return () => {
			cancelAnimationFrame(frameId);
			window.removeEventListener('resize', handleResize);
			renderer.dispose();
			geometry.dispose();
			material.dispose();
		};
	});
</script>

<canvas bind:this={canvas} class="fixed inset-0 bg-black"></canvas>
