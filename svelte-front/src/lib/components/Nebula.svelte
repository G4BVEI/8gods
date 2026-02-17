<script>
	import { onMount } from 'svelte';
	import * as THREE from 'three';

	let canvas;
	let renderer, scene, camera, material;
	let frameId;

	const fragmentShader = `
    uniform vec3 iResolution;
    uniform float iTime;

    // --- Arcane Tweaks ---
    #define iterations 8
    #define formuparam 0.85
    #define volsteps 13
    #define stepsize 0.120

    #define zoom 2.500
    #define tile 0.850
    #define brightness 0.0015
    #define darkmatter 0.400
    #define distfading 0.650
    #define saturation 0.850

    #define cloud 0.15

    float field(in vec3 p) {
      float strength = 7.0 + 0.05 * sin(iTime * 2.0);
      float accum = 0.;
      float prev = 0.;
      float tw = 0.;
      for (int i = 0; i < 6; ++i) {
        float mag = dot(p, p);
        p = abs(p) / mag + vec3(-.5, -.8 + 0.1 * sin(iTime * 0.1 + 2.0), -1.1 + 0.3 * cos(iTime * 0.15));
        float w = exp(-float(i) / 7.2);
        accum += w * exp(-strength * pow(abs(mag - prev), 2.2));
        tw += w;
        prev = mag;
      }
      return max(0., 5. * accum / tw - .7);
    }

    void main() {
      vec2 uv2 = 2. * gl_FragCoord.xy / iResolution.xy - 1.;
      vec2 uvs = uv2 * iResolution.xy / max(iResolution.x, iResolution.y);

      float time2 = iTime * 0.8;
      // Speed kept slightly higher for that "Space" feel
      float speed = 0.03 * cos(time2 * 0.02);

      float a_xz = 0.9;
      float a_yz = -.6;
      float a_xy = 0.5 + iTime * 0.01;

      mat2 rot_xz = mat2(cos(a_xz),sin(a_xz),-sin(a_xz),cos(a_xz));
      mat2 rot_yz = mat2(cos(a_yz),sin(a_yz),-sin(a_yz),cos(a_yz));
      mat2 rot_xy = mat2(cos(a_xy),sin(a_xy),-sin(a_xy),cos(a_xy));

      vec3 dir = vec3(uvs * zoom, 1.);
      vec3 from = vec3(0.0, 0.0, 0.0);
      vec3 forward = vec3(0.,0.,1.);

      from.x += 1.2 * cos(0.01 * iTime);
      from.y += 1.2 * sin(0.01 * iTime);
      from.z += 0.003 * iTime;

      dir.xy *= rot_xy;
      forward.xy *= rot_xy;
      dir.xz *= rot_xz;
      forward.xz *= rot_xz;
      dir.yz *= rot_yz;
      forward.yz *= rot_yz;

      from.xy *= -rot_xy;
      from.xz *= rot_xz;
      from.yz *= rot_yz;

      float zooom = (time2-3311.)*speed;
      from += forward * zooom;
      float sampleShift = mod( zooom, stepsize );
      float zoffset = -sampleShift;
      sampleShift /= stepsize;

      float s=0.24;
      float s3 = s + stepsize/2.0;
      vec3 v = vec3(0.);
      vec3 backCol2 = vec3(0.);

      for (int r=0; r<volsteps; r++) {
        vec3 p2 = from + (s + zoffset) * dir;
        vec3 p3 = (from + (s3 + zoffset) * dir ) * (1.5 / zoom);

        p2 = abs(vec3(tile) - mod(p2, vec3(tile * 2.)));
        p3 = abs(vec3(tile) - mod(p3, vec3(tile * 2.)));

        float t3 = field(p3);
        float pa, a = pa = 0.;
        for (int i=0; i<iterations; i++) {
          p2 = abs(p2) / dot(p2, p2) - formuparam;
          float D = abs(length(p2) - pa);
          if (i > 3) a += D;
          pa = length(p2);
        }

        a *= a * a;
        float s1 = s + zoffset;

        // --- DISTANCE CULLING LOGIC ---
        // We create a mask that goes from 1.0 (close) to 0.0 (far)
        float distMask = pow(1.0 - (float(r) / float(volsteps)), 2.5);
        float fade = pow(distfading, max(0., float(r) - sampleShift)) * distMask;

        v += fade;
        if( r == 0 ) fade *= (1. - (sampleShift));
        if( r == volsteps-1 ) fade *= 0.0; // Force distant step to 0

        v += vec3(s1 * 0.5, s1 * s1, s1 * s1 * s1) * a * brightness * fade;
        backCol2 += mix(.3, 1., 0.8) * vec3(0.4 * t3 * t3, 0.1 * t3, t3 * 0.8) * fade;

        s += stepsize;
        s3 += stepsize;
      }

      v = mix(vec3(length(v)), v, saturation);
      vec4 forCol2 = vec4(v * .01, 1.);
      backCol2 *= cloud;

      vec3 finalCol = forCol2.rgb + backCol2;
      gl_FragColor = vec4(finalCol, 1.0);
    }
  `;

	onMount(() => {
		scene = new THREE.Scene();
		camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);
		renderer = new THREE.WebGLRenderer({ canvas, antialias: false });
		renderer.setSize(window.innerWidth, window.innerHeight);

		const geometry = new THREE.PlaneGeometry(2, 2);
		material = new THREE.ShaderMaterial({
			fragmentShader,
			uniforms: {
				iTime: { value: 0 },
				iResolution: { value: new THREE.Vector3(window.innerWidth, window.innerHeight, 1) }
			}
		});

		scene.add(new THREE.Mesh(geometry, material));

		const clock = new THREE.Clock();
		const animate = () => {
			frameId = requestAnimationFrame(animate);
			material.uniforms.iTime.value = clock.getElapsedTime();
			renderer.render(scene, camera);
		};

		animate();

		const handleResize = () => {
			renderer.setSize(window.innerWidth, window.innerHeight);
			material.uniforms.iResolution.value.set(window.innerWidth, window.innerHeight, 1);
		};
		window.addEventListener('resize', handleResize);

		return () => {
			cancelAnimationFrame(frameId);
			window.removeEventListener('resize', handleResize);
			renderer.dispose();
		};
	});
</script>

<canvas bind:this={canvas} class="fixed inset-0 bg-black"></canvas>
