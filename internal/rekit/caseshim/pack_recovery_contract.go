package caseshim

import (
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

type PackRecoveryWrite struct {
	Kind            string
	AcceptedSHA256s map[string]struct{}
}

func generation(kind string, sums ...string) PackRecoveryWrite {
	accepted := map[string]struct{}{}
	for _, sum := range sums {
		accepted[sum] = struct{}{}
	}
	return PackRecoveryWrite{Kind: kind, AcceptedSHA256s: accepted}
}

// packRecoveryWrites is append-only by path and generation hash. Removing an
// accepted generation would make an already-published pending intent unrecoverable.
var packRecoveryWrites = map[string]map[string]PackRecoveryWrite{
	"_template": {
		"references/template/README.md":            generation("managed-file", "5267981c0cf3c9f625f563c94a25ab4ade2db7b52748a6e219a42d9c42fcba8a"),
		"references/template/agent-team.md":        generation("managed-file", "d77f56ec15028c8185f9fe696ff8c1168c00b908a13b05108d266076a66397c3"),
		"references/template/workflow-template.md": generation("managed-file", "bce85634f9fc6a8f559476116342855af0c4eff1826cef7468e31fe76836fd5b", "fd4e9d8bb26217fc3831e2810c7104cd10b93d08a85c38512e7c2e0d3524024c"),
		"references/template/toolchain-router.md":  generation("managed-file", "b0e75758e4a7a9542c1a741e92b671c699b6597a9555250c5899461a089db162"),
		"references/template/task-handoff.md":      generation("template-file", "60e54a4187e1af7f87efac80d6de843842f8c0d09cccce35462bdf20978e7394"),
	},
	defaults.DefaultPack: defaultPackRecoveryWrites(),
	"web-security":       packSkeletonWithCanonical("web-security", "0346c14f7fb46023ecce4ac53cb2069df6b42350d5028442d1d293ef2d62c7ad", "c4f46ac2e5907c2460ed3618114501942ce917e85a441ab6e32552f438af7a17", "429bdcef398e6fef79e7ec1894a2df243bd92d7b37c30c42cb1dfc12935ce370", "f92a598fe5756c51e0fe89c9e6ae3d4cb0110fa56fd10ec1f97d5ec5e1ad6f4b", "c1f15a2f913fbde7e02d81fbefc99eff830deee223768decda541e45df6c907a", "48afc30652f7751483ad972d544eba3e1952b31721ec77f8b0d76dc08462db7d", "15f56e64494e54ed3da60434ff991a9c7a2e73b2fd691d714b34f843105e600e", "6eae98db08786d52c73fefb02e8bad7d89becf04421e7aa4f9ecc2ea58c0d79e"),
	"malware-analysis":   packSkeletonWithCanonical("malware-analysis", "07015df0d0384714a8df3ba4c7e8b1eba3db318618f82705ae9381a7958c7210", "929b8cde6da7bc169ef8678b74e496a9af4795f02731925931e4999c5a4c7614", "670bdad792de8d24576561d3cb27aee88bf182981bf2e6253a721bf746e690d2", "461b40e924420e98ae0483ff84f0632a23dd077ddb8162649bed007d55e06bd0", "ebd0b52b6d76e3da8c3c8bddafab535077deb02be98f07fcb554eedfa5b534a7", "52331a59b2ceff6068ddf636e928f72d3960b3a1286b81aa230d8c27cf5006bc", "b2548fd32202cfa6ec1672615c74435106603763922180673a2d411f9a5aeb34", "7a8b7fc12ff8ccc6a3ce1799505386d28adde4d9f684c6d87492a840ba2c68ec"),
	"vuln-research":      packSkeletonWithCanonical("vuln-research", "5057a3ac36ea77c8244fa8781f853d5f66299d1fc5f307b483e5a18031627cf9", "fb1e2d807985d68b57ab92d6c963eef62f375d8f7f7d44cfc14e2a207dcf379c", "05ce1f25ca5ca82188ded9ba05138a06d41dfa7a45a3ca85bb261d6791d86cfa", "c52eba6759a7a08d76e89fea7a86055899f8ab13f5edc290d193745e17b0a516", "037daa8c0071bea5bf8d1995ac2d5ed3df4a5b27c249e6a3eabd3032d149869b", "fb1c23f85fd7d49f11152617cd39fba12c6e9c5abef7c52d8d1e595858b5cfb4", "36c3dc75fd82907977587bbf5a46e4883e277eb71b50d7f9559aa3daa1e9b4dc", "36bbb6b64f3886ece06e6abe4ca61b4fb1d2fb213df8db60fd9aed01c611902a"),
	"ctf":                packSkeletonWithCanonical("ctf", "a4876bf594b4298cfa4126892a087858560036acaab9ceb8b697255a1228c347", "3480c3660a88b7c84e18dc53f17356a08f59b5c28248a22d3399a2d86a7a2f17", "1cbab22120efcad34c0e051fb090d9a20da7724d744f78e94be5d88b242500d8", "7c5e2d72f90ba7b1804063a61c3d83d58da288d6e346037ca97457363cafadc0", "16fa8a9a2520f3b5020310eb2520bfdc3b33064f584de1b8c2f1356eb4ccdb9e", "843d3192d161ade743660712bb0169e01dc18afb235cb7e56be1494332ef40b8", "eb30ddb9e6cbeba162cf3e6ceb98e48c3130def4bc40494e7dcc99775ecd71ec", "1b8d49ec05b7a00c29156f8b5b6f67b6f1fa31b00b75e69f95f0a6a526f5245b"),
	"unpack-pe":          packSkeletonWithCanonical("unpack-pe", "4535e384b38fc4e57514deb929923e69901838a334d4bf7b8481bd022e4bea9a", "a173c53b9ca04bc6ec192ddc32950d021ce651faf68668dd227cff939169b72f", "1a164d8ea9df0239698e8504672cc55de8b022e2578bb98041e9c4eaf5a1197b", "9d1e6467d4f533676d2254fa162af077a900ec4978d4aab0fd00138c85a6c8a7", "4c02569d536bfefccc0cdc53cfe5ec4a2b4159bd97995774bdf5c62afaec92b1", "d90cd2f02a13d09bddfd68738aff6e2ed2d4f8e0d03bd72d2c1f4189819360cd", "7ea1fe0889a23e28ad79d25821b8a82619b72331f345358ab81a41c1ad31668f", "471860a73f378eb802162ce27fb5f27df31f5c9f8babc08ed72b7f3e9beae827"),
	"ollvm":              packSkeletonWithCanonical("ollvm", "2d6cf394c5c40e169fc93fdd0f2904821c8573161539dee3dfadd6d5223652d6", "13d13482fbbb255e696221bc702dcda3068d0c40a10f60ca2cb3b02e54d2ed9f", "57e2e9a6439bb8c6092cfc12afe41df8580ce9a8829f73b802635ada29ec1030", "275242801a385416ecbd85379a2712c0531628e3c7f6dae2b5a188662954cccb", "e6a2e10b8e994f13a04a451a6b2750bb1896b422858826755fbdc9d50f1a0255", "a9032b85cd8a2182d7f3074cfc0be795dc830a81787cfbe8bf9f1876b1bc1da1", "becc846162602415a4dcbe18def33c8ce191c0269f95979097ad15309ee5d164", "0b2f6a462e546afd8fe7b5d5e99fcd271f3d689ba7677336f5adae7989d43e53"),
	"android-native":     packSkeletonWithCanonical("android-native", "30ea2c839cabfe28a8a426f3b053751bf449c23370fba097acc993affd0c3879", "f936dab11ef4cff8c73fd9528b3b0783c9cafb1157ecac7808d7f2ea2ba67e34", "5906465e67d8ee08d647072e5800e5ec7067068bf8234bdbaf954dbd77e5a135", "da385ffda2f1308d521de716369f83f94fb8d394ad3d27bac76c83d3177a7b73", "553dceb194067d7970fd361e792f94f741a8d5794aad8f9769f316d04a49d3ec", "e2c9cbd3a8d6fb7fd556ed6acc4806d6e402fe244b052db58f363143029797e7", "7e6fc542b3a51f3851a0397fae536ebe4ba549011a839d6a82ab8db5285cced7", "303d14c58f9231f7d2f8b6829fbc8afa627457aa0ee26b09c3c3a731ba54d968"),
	"generic-binary-re":  packSkeletonWithCanonical("generic-binary-re", "91883aff98b8c9817b7c789b40072f1e2bd5e551aae3db62c5abdce60a52efc8", "c6a79a32eeb9b996a259f763e4f8dd4b62924d1345fb46f4673ad5251cd7b747", "51651baf2364e08e201d2c34144be523e302af26ef1355ac90385a280d657bda", "0430286b030814b7485721bbae2b8219309051d117c88f4592e354cb599afdb7", "aad3f5d98208b478b5fafaa4a982baa9c534de7aea5ba8d638f165facf948427", "12b983b75d2019b9a1a12751f9d72083d048ae91ff97dc1d4c5629f9e33d0b7a", "0107c0198bab6aa6a2b9a26409b154133e903cbdf7c49932b384df11c8f4c515", "2f9c246f7ff497b656fd8b421346f6ea534a535794eaebe04383488db29699ba"),
}

func defaultPackRecoveryWrites() map[string]PackRecoveryWrite {
	prefix := "references/" + defaults.DefaultPack + "/"
	return map[string]PackRecoveryWrite{
		prefix + "README.md":                   generation("managed-file", "dbb5dcf99256d2ef8fecd46ea573483299f6547c7359c39b222aa41bb1f08a70"),
		prefix + "agent-driven-re.md":          generation("managed-file", "b52e2f367db70034f09b28820e1f99924987a3a9c21f4abd092c443910b24bf8"),
		prefix + "workflow-template.md":        generation("managed-file", "ea9b72db6a3820a6f2e5708a9a8d8afac5fc203b58a03fe232a5719b88198c61"),
		prefix + "progressive-disclosure.md":   generation("managed-file", "468ef2e1aa6d2bea6167bbdb8b163fcc8588661883ba6452e23c03fc7c864331"),
		prefix + "toolchain-router.md":         generation("managed-file", "816628c21cfa7df813a8c730486c9da0a7fc3f452affdf15d8405be94f750b9e"),
		prefix + "singleton-handler-review.md": generation("managed-file", "cbd2cae562d09e9a14f07454b5eb20c5d927a3d587670b571c2b00bc145dd23c"),
		prefix + "lane-collaboration.md":       generation("managed-file", "18d3963aefb6f8e9bd85c17724a12220451162b50c364e5e68ff488ca8c167b9"),
		prefix + "task-handoff.md":             generation("template-file", "6960d6ca8fad6697575c767b85e0acd8b49d5539c180b46921fdbde76ea94aa6"),
	}
}

func packSkeletonWithCanonical(pack, readme, agent, workflow, router, handoff, canonicalReadme, canonicalWorkflow, canonicalHandoff string) map[string]PackRecoveryWrite {
	prefix := "references/" + pack + "/"
	return map[string]PackRecoveryWrite{
		prefix + "README.md":            generationWithCanonical("managed-file", readme, canonicalReadme),
		prefix + "agent-team.md":        generation("managed-file", agent),
		prefix + "workflow-template.md": generationWithCanonical("managed-file", workflow, canonicalWorkflow),
		prefix + "toolchain-router.md":  generation("managed-file", router),
		prefix + "task-handoff.md":      generationWithCanonical("template-file", handoff, canonicalHandoff),
	}
}

func generationWithCanonical(kind, current, canonical string) PackRecoveryWrite {
	if canonical == "" {
		return generation(kind, current)
	}
	return generation(kind, current, canonical)
}

func PackRecoveryWrites(pack string) map[string]PackRecoveryWrite {
	return packRecoveryWrites[pack]
}

func ValidatePackRecoveryWrite(pack, path, kind, normalizedSHA256 string) error {
	contract, ok := packRecoveryWrites[pack][path]
	if !ok || contract.Kind != kind {
		return fmt.Errorf("write path/kind is not in the trusted pack recovery contract")
	}
	if _, ok := contract.AcceptedSHA256s[normalizedSHA256]; !ok {
		return fmt.Errorf("write content does not match an accepted trusted pack generation")
	}
	return nil
}
