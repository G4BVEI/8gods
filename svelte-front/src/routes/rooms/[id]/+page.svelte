<script>
	import ShaderRunner from '$lib/engine/ShaderRunner.svelte';
	let banana = '';
</script>

<div class="flex h-dvh overflow-hidden">
	<div class="left-0 h-dvh w-[30dvw] bg-gray-600">
		<button onclick={() => banana++}>banana</button>
	</div>
	<div class="right-0 h-dvh w-[70dvw]">
		<ShaderRunner />
	</div>
</div>
