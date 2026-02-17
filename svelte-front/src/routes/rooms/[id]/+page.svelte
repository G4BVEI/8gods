<script>
	import Space from '$lib/components/Space.svelte';
</script>

<!-- Example 1: Full viewport background -->
<Space />

<div style="width: 100vw; height: 200vh;"></div>

<style>
	/* Example 1: Full viewport */
	.game-background {
		width: 100vw;
		height: 100vh;
		position: fixed;
		top: 0;
		left: 0;
	}

	/* Content styling */
	.content-overlay {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: white;
		text-align: center;
		padding: 2rem;
	}

	h1 {
		font-size: 3rem;
		margin-bottom: 1rem;
		text-shadow: 0 0 20px rgba(0, 255, 240, 0.5);
	}

	p {
		font-size: 1.2rem;
		max-width: 600px;
		line-height: 1.6;
	}
</style>
