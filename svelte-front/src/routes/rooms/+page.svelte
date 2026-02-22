<script>
	import ShaderRunner from '$lib/engine/ShaderRunner.svelte';
	const rooms = [
		{
			id: 1,
			Nome: 'Sala 1',
			Jogadores: 12,
			Senha: false
		},
		{
			id: 2,
			Nome: 'Sala 1',
			Jogadores: 12,
			Senha: false
		},
		{
			id: 3,
			Nome: 'Sala 1',
			Jogadores: 12,
			Senha: false
		}
	];
</script>

<!-- Just drop it in once - it becomes the page background -->
<div class="fixed -z-1 h-dvh w-dvw"><ShaderRunner /></div>

<!-- Your normal scrollable content -->
<main>
	<section class="min-h-[20dvh]">
		<h1>Welcome to Hyperspace</h1>
		<p>Scroll down to see the background stay fixed</p>
	</section>
	<div></div>
	<p>peido</p>
	{#each rooms as room (room.id)}
		<section class="content-section">
			<h2>{room.Nome}</h2>
			<p>
				{room.jogadores}
			</p>
			{#if (room.senha = true)}
				<p>Tem senha</p>
			{:else}
				<p>Não tem senha</p>
			{/if}
		</section>
	{/each}
	<section class="content-section">
		<h2>Section 2</h2>
		<p>No need to wrap your content in anything - just place the component once in your layout.</p>
		<div class="card">
			<h3>Feature Card</h3>
			<p>All your content is interactive and scrollable</p>
		</div>
	</section>

	<footer>
		<p>End of page - background still animating behind</p>
	</footer>
</main>

<style>
	:global(body) {
		margin: 0;
		padding: 0;
		color: white;
		font-family:
			system-ui,
			-apple-system,
			sans-serif;
	}

	main {
		position: relative;
		z-index: 1;
		min-height: 300vh; /* Make it scrollable */
	}

	section {
		padding: 4rem 2rem;
		color: white;
	}

	.hero {
		min-height: 20dvh;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		text-align: center;
	}

	.hero h1 {
		font-size: 4rem;
		margin: 0 0 1rem 0;
		text-shadow: 0 0 20px rgba(0, 255, 240, 0.5);
	}

	.hero p {
		font-size: 1.5rem;
		opacity: 0.8;
	}

	.content-section {
		max-width: 800px;
		margin: 0 auto;
		background: rgba(0, 0, 0, 0.3);
		border-radius: 12px;
		backdrop-filter: blur(10px);
		border: 1px solid rgba(255, 255, 255, 0.1);
		margin-bottom: 2rem;
	}

	.content-section h2 {
		font-size: 2.5rem;
		margin-top: 0;
		color: rgba(0, 255, 240, 0.9);
		text-shadow: 0 0 10px rgba(0, 255, 240, 0.3);
	}

	.content-section p {
		font-size: 1.2rem;
		line-height: 1.8;
		opacity: 0.9;
	}

	.card {
		background: rgba(150, 50, 255, 0.1);
		border: 1px solid rgba(150, 50, 255, 0.3);
		border-radius: 8px;
		padding: 2rem;
		margin-top: 2rem;
	}

	.card h3 {
		margin-top: 0;
		color: rgba(150, 50, 255, 1);
		text-shadow: 0 0 10px rgba(150, 50, 255, 0.3);
	}

	button {
		margin-top: 1rem;
		padding: 1rem 2rem;
		font-size: 1.1rem;
		background: rgba(0, 255, 240, 0.2);
		border: 2px solid rgba(0, 255, 240, 0.5);
		color: white;
		cursor: pointer;
		transition: all 0.3s ease;
		border-radius: 4px;
	}

	button:hover {
		background: rgba(0, 255, 240, 0.3);
		box-shadow: 0 0 20px rgba(0, 255, 240, 0.4);
		transform: translateY(-2px);
	}

	footer {
		text-align: center;
		padding: 4rem 2rem;
		color: rgba(255, 255, 255, 0.6);
		font-size: 1.1rem;
	}
</style>
