<script>
	import CyanNebula from '$lib/components/CyanNebula.svelte';
	import Nebula from '$lib/components/Nebula.svelte';
	import NeonNebula from '$lib/components/NeonNebula.svelte';
	let banana = $state(0);
</script>

<div class="flex h-dvh overflow-hidden">
	<div class="left-0 h-dvh w-[35dvw] bg-gray-600">
		<button onclick={() => banana++}>banana</button>
	</div>
	<div class="right-0 h-dvh w-[65dvw]">
		{#if banana % 3 == 0}
			<Nebula />
		{:else if banana % 3 == 1}
			<CyanNebula />
		{:else}
			<NeonNebula />
		{/if}
	</div>
</div>
