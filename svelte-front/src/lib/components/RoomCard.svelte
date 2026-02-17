<script>
	let { room } = $props();
</script>

<section class="content-section">
	<h2>{room.title}</h2>
	<p>
		{room.description}
	</p>
	{#if (room.senha = true)}
		<p>Tem senha</p>
	{:else}
		<p>Não tem senha</p>
	{/if}
</section>

<style>
	.content-section {
		max-width: 800px;
		margin: 0 auto;
		background: rgba(0, 0, 0, 0.3);
		border-radius: 12px;
		backdrop-filter: blur(10px);
		border: 1px solid rgba(255, 255, 255, 0.1);
		margin-bottom: 2rem;
	}
</style>
