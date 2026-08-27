package caseshim

import (
	"fmt"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
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

// Repository identity changes only the current README generation. Keep both
// source and canonical publication bytes so pending legacy and current intents
// remain recoverable across checkout line-ending representations.
var repositoryIdentityReadmeGenerations = map[string][]string{
	"web-security":              {"b46e8838c2bffe068ea1924cd44429dfe91b1f1ef43b5d06acc5c2727d6ea206", "751918b6ed75804f122af5e3bb8e2a78c53f58a04b1a61203eeb0716492483b5", "1fa43fc4592b60250f44dad53bc0c9e96873d6dd0a628279ed29904ab31475d8", "a16b0980b0fd75ac2e413c4969c8e3215bc694c838b836763a6c1a235f783c19"},
	"malware-analysis":          {"2ecdaa3341d6bb149a6874ca075eb021d6d9c23cc1ba3592b3301114e9968547", "df9138719fba90db867072c3cc8b605493aa492d134625c0e8a64d0caf0fe3bc", "cbff9e3b90e2f0ee8aa3fab11ab98df2b1dd09ac4d059c1ef6f847588cd88ac3", "b14f8f8aba9c9500fa8eed168ad6d52d3a290a63bd97548a0bdc4612b95202a7"},
	"vuln-research":             {"1f90eebfe35e46264c7e44bfb8e616381ddb3d46eb9cbb10b31bde3d295b5896", "8601faedb4f3014bf369b4397d43539782072997b00f4e138a198e1edad2c2cd", "f3a7474f1780ed683a3a7160309a84b2a48334f7a16cd01e271de777179b8891", "4c1440cdb297312ff74c4f32598845ad95b45535df9d71fb66e47197bf5e51dc"},
	"ctf":                       {"ec1ad3c131b863799cff6488b0f7e80b88398880caf2f99b0a8ebd33ace3e6d0", "051f056f72d88b1bc0f189dc4827aee693ed5d0c6037b03efbfbc3a2cbc32526", "823548f858e78ed761528b76851c256204567cee05954de20b1a45c7c8b62d27", "2d0ce676b42e4033c3e7a5960ea7194c462b7bbece3c50b4d0bf944c37e680be"},
	"unpack-pe":                 {"2769228bbbd50646712b0be2c22412e2e0b8c704686d71fa31ffbd6673b8b9bc", "226c2c8d7c4fa5e7258655637aa17f1e3feea2abdbcea9535f25f0ecc6c3b100", "fb51756eb038716393c3b578f7d2624dbd53e2e13692a490627497c64b751b74", "def680207f5f216e86367f49a7f9318764e7113229f102b4999e5f48b74c915f"},
	"ollvm":                     {"9683acc99b1bf35e74b0371c7b87d74b1f7f6c510b55ff64cc76f4f0795c9548", "8794c807a6be20c20f330c926cbaae7e760e0c33adab854e4ae27b0dafc18de8", "17d8a0beb28f27203690531ced363f3f9f206bab68085c22f872049bc506329e", "6826bd35919d9637c5379fdfb9ca7771ed9dedb2cd908f62f1fe96b8479dfb80"},
	"android-native":            {"1006878baf2a91032b39dc890262ae795325696ad5a1b7224a141567c3585331", "23098f039871e33d0ceab2e576534da518a5f97d3ff95f4e55a6c2e595a9cbb9", "c16d93bd5f21605587ddefd04e25cff2968f5a42d3fbbd57d1da24ab38e822f6", "03e3e0de7cd83d310ddd13d08a1fbb96ebdc27d54c7298ac1e6e225186747738"},
	packidentity.RetiredGeneric: {"744bff97810b86a998402e40f16070c6fe48cd880756ce247e91cc4b78531e9a", "896c780549c096d3b420a32422e30d643f5c48d94920725b9d3feaa4f1c4e3ed"},
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
	defaults.DefaultPack:        defaultPackRecoveryWrites(),
	"web-security":              webSecurityRecoveryWrites(),
	"malware-analysis":          packSkeletonWithCanonical("malware-analysis", "07015df0d0384714a8df3ba4c7e8b1eba3db318618f82705ae9381a7958c7210", "929b8cde6da7bc169ef8678b74e496a9af4795f02731925931e4999c5a4c7614", "670bdad792de8d24576561d3cb27aee88bf182981bf2e6253a721bf746e690d2", "461b40e924420e98ae0483ff84f0632a23dd077ddb8162649bed007d55e06bd0", "ebd0b52b6d76e3da8c3c8bddafab535077deb02be98f07fcb554eedfa5b534a7", "52331a59b2ceff6068ddf636e928f72d3960b3a1286b81aa230d8c27cf5006bc", "b2548fd32202cfa6ec1672615c74435106603763922180673a2d411f9a5aeb34", "7a8b7fc12ff8ccc6a3ce1799505386d28adde4d9f684c6d87492a840ba2c68ec"),
	"vuln-research":             packSkeletonWithCanonical("vuln-research", "5057a3ac36ea77c8244fa8781f853d5f66299d1fc5f307b483e5a18031627cf9", "fb1e2d807985d68b57ab92d6c963eef62f375d8f7f7d44cfc14e2a207dcf379c", "05ce1f25ca5ca82188ded9ba05138a06d41dfa7a45a3ca85bb261d6791d86cfa", "c52eba6759a7a08d76e89fea7a86055899f8ab13f5edc290d193745e17b0a516", "037daa8c0071bea5bf8d1995ac2d5ed3df4a5b27c249e6a3eabd3032d149869b", "fb1c23f85fd7d49f11152617cd39fba12c6e9c5abef7c52d8d1e595858b5cfb4", "36c3dc75fd82907977587bbf5a46e4883e277eb71b50d7f9559aa3daa1e9b4dc", "36bbb6b64f3886ece06e6abe4ca61b4fb1d2fb213df8db60fd9aed01c611902a"),
	"ctf":                       packSkeletonWithCanonical("ctf", "a4876bf594b4298cfa4126892a087858560036acaab9ceb8b697255a1228c347", "3480c3660a88b7c84e18dc53f17356a08f59b5c28248a22d3399a2d86a7a2f17", "1cbab22120efcad34c0e051fb090d9a20da7724d744f78e94be5d88b242500d8", "7c5e2d72f90ba7b1804063a61c3d83d58da288d6e346037ca97457363cafadc0", "16fa8a9a2520f3b5020310eb2520bfdc3b33064f584de1b8c2f1356eb4ccdb9e", "843d3192d161ade743660712bb0169e01dc18afb235cb7e56be1494332ef40b8", "eb30ddb9e6cbeba162cf3e6ceb98e48c3130def4bc40494e7dcc99775ecd71ec", "1b8d49ec05b7a00c29156f8b5b6f67b6f1fa31b00b75e69f95f0a6a526f5245b"),
	"unpack-pe":                 packSkeletonWithCanonical("unpack-pe", "4535e384b38fc4e57514deb929923e69901838a334d4bf7b8481bd022e4bea9a", "a173c53b9ca04bc6ec192ddc32950d021ce651faf68668dd227cff939169b72f", "1a164d8ea9df0239698e8504672cc55de8b022e2578bb98041e9c4eaf5a1197b", "9d1e6467d4f533676d2254fa162af077a900ec4978d4aab0fd00138c85a6c8a7", "4c02569d536bfefccc0cdc53cfe5ec4a2b4159bd97995774bdf5c62afaec92b1", "d90cd2f02a13d09bddfd68738aff6e2ed2d4f8e0d03bd72d2c1f4189819360cd", "7ea1fe0889a23e28ad79d25821b8a82619b72331f345358ab81a41c1ad31668f", "471860a73f378eb802162ce27fb5f27df31f5c9f8babc08ed72b7f3e9beae827"),
	"ollvm":                     packSkeletonWithCanonical("ollvm", "2d6cf394c5c40e169fc93fdd0f2904821c8573161539dee3dfadd6d5223652d6", "13d13482fbbb255e696221bc702dcda3068d0c40a10f60ca2cb3b02e54d2ed9f", "57e2e9a6439bb8c6092cfc12afe41df8580ce9a8829f73b802635ada29ec1030", "275242801a385416ecbd85379a2712c0531628e3c7f6dae2b5a188662954cccb", "e6a2e10b8e994f13a04a451a6b2750bb1896b422858826755fbdc9d50f1a0255", "a9032b85cd8a2182d7f3074cfc0be795dc830a81787cfbe8bf9f1876b1bc1da1", "becc846162602415a4dcbe18def33c8ce191c0269f95979097ad15309ee5d164", "0b2f6a462e546afd8fe7b5d5e99fcd271f3d689ba7677336f5adae7989d43e53"),
	"android-native":            packSkeletonWithCanonical("android-native", "30ea2c839cabfe28a8a426f3b053751bf449c23370fba097acc993affd0c3879", "f936dab11ef4cff8c73fd9528b3b0783c9cafb1157ecac7808d7f2ea2ba67e34", "5906465e67d8ee08d647072e5800e5ec7067068bf8234bdbaf954dbd77e5a135", "da385ffda2f1308d521de716369f83f94fb8d394ad3d27bac76c83d3177a7b73", "553dceb194067d7970fd361e792f94f741a8d5794aad8f9769f316d04a49d3ec", "e2c9cbd3a8d6fb7fd556ed6acc4806d6e402fe244b052db58f363143029797e7", "7e6fc542b3a51f3851a0397fae536ebe4ba549011a839d6a82ab8db5285cced7", "303d14c58f9231f7d2f8b6829fbc8afa627457aa0ee26b09c3c3a731ba54d968"),
	packidentity.RetiredGeneric: packSkeletonWithCanonical(packidentity.RetiredGeneric, "91883aff98b8c9817b7c789b40072f1e2bd5e551aae3db62c5abdce60a52efc8", "c6a79a32eeb9b996a259f763e4f8dd4b62924d1345fb46f4673ad5251cd7b747", "51651baf2364e08e201d2c34144be523e302af26ef1355ac90385a280d657bda", "0430286b030814b7485721bbae2b8219309051d117c88f4592e354cb599afdb7", "aad3f5d98208b478b5fafaa4a982baa9c534de7aea5ba8d638f165facf948427", "12b983b75d2019b9a1a12751f9d72083d048ae91ff97dc1d4c5629f9e33d0b7a", "0107c0198bab6aa6a2b9a26409b154133e903cbdf7c49932b384df11c8f4c515", "2f9c246f7ff497b656fd8b421346f6ea534a535794eaebe04383488db29699ba"),
	packidentity.RetiredVMP:     retiredVMPRecoveryWrites(),
}

// retiredVMPRecoveryWrites preserves the pre-cutover vmp-re publication
// contract. It intentionally does not derive paths or hashes from the
// canonical binary-re defaults: the retired identity is recoverable only when
// its original path and generation bytes match this explicit contract.
func retiredVMPRecoveryWrites() map[string]PackRecoveryWrite {
	const prefix = "references/vmp-re/"
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

func defaultPackRecoveryWrites() map[string]PackRecoveryWrite {
	prefix := "references/" + defaults.DefaultPack + "/"
	return map[string]PackRecoveryWrite{
		prefix + "README.md":                   generation("managed-file", "dbb5dcf99256d2ef8fecd46ea573483299f6547c7359c39b222aa41bb1f08a70", "d9303c2ae1cb48d8fcd24c269e025020941228aad4fe2215a90e70344b8bee5a", "0438b4d5ba6f76ad609c93b3127593f706865c166f5fb18953e7a09f5d6472e6", "f5fe89f502a555a9dcd6700cdb1a905cea023742f4a150050b55e072f1ecc239"),
		prefix + "agent-driven-re.md":          generation("managed-file", "b52e2f367db70034f09b28820e1f99924987a3a9c21f4abd092c443910b24bf8", "004196bacabfcfa644e22f9e87c2234c4420d24ce8a9338953594ca9b1be081f"),
		prefix + "workflow-template.md":        generation("managed-file", "ea9b72db6a3820a6f2e5708a9a8d8afac5fc203b58a03fe232a5719b88198c61", "795e22978f407e6c0ca48cb17341099bb90e5b3ea0602bf79ee306cf99e64d5a"),
		prefix + "progressive-disclosure.md":   generation("managed-file", "468ef2e1aa6d2bea6167bbdb8b163fcc8588661883ba6452e23c03fc7c864331", "a583693c67c71edbb4ecf1c58bab3ab99cb58b387d1bdaaffe64ad226d2bf6a7", "c562a24f8c944bd894f60e526248cf87f1c4c74c7ff9636e6e92958b3decb5d1"),
		prefix + "toolchain-router.md":         generation("managed-file", "816628c21cfa7df813a8c730486c9da0a7fc3f452affdf15d8405be94f750b9e", "c2ac4a190674c2cfebfd25c8344b83171eb26a5bcc62573ff5fc0935a5816b57"),
		prefix + "singleton-handler-review.md": generation("managed-file", "cbd2cae562d09e9a14f07454b5eb20c5d927a3d587670b571c2b00bc145dd23c"),
		prefix + "lane-collaboration.md":       generation("managed-file", "18d3963aefb6f8e9bd85c17724a12220451162b50c364e5e68ff488ca8c167b9", "fa36ab439290c9f747c5e015aba1d27d5784bd17b7cacd54d03400f6377d974f", "941ec5a3a625cd61615be8ea948e2aa9b2970d70cffa37cbb609d9e49b031ab6"),
		prefix + "general-analysis.md":         generation("managed-file", "c6f70c9f15576402f563aaa190413fd3902463ee05ee6baa2b8da86b4a5f011c", "0b905526edf8b6ac9e68e41898c0375554787c7cf1b1e29fa53c7d78a1581018", "f91a5b0813ab20ed87fa7ed4d50748b65abc249a8be96879f808b6567a3639eb"),
		prefix + "general-agent-team.md":       generation("managed-file", "b148397a565bf81468599d033bc5c3e76b80ffa8441cef029478d10ae2ab01ce"),
		prefix + "general-workflow.md":         generation("managed-file", "368efcf14d0c20814c50ed32e7309a7dafb725109c5a4f76e283681283c5430e"),
		prefix + "general-toolchain-router.md": generation("managed-file", "e1f94327afb948713fb942b4e6b9c090c329156de7e3438857c48566685e2c2c"),
		prefix + "task-handoff.md":             generation("template-file", "6960d6ca8fad6697575c767b85e0acd8b49d5539c180b46921fdbde76ea94aa6", "1c8cc6db4e29b1864cf58545afcf91727fde1e40ff7596beaedf7efadd953c99"),
	}
}

func webSecurityRecoveryWrites() map[string]PackRecoveryWrite {
	writes := packSkeletonWithCanonical("web-security", "0346c14f7fb46023ecce4ac53cb2069df6b42350d5028442d1d293ef2d62c7ad", "c4f46ac2e5907c2460ed3618114501942ce917e85a441ab6e32552f438af7a17", "429bdcef398e6fef79e7ec1894a2df243bd92d7b37c30c42cb1dfc12935ce370", "f92a598fe5756c51e0fe89c9e6ae3d4cb0110fa56fd10ec1f97d5ec5e1ad6f4b", "c1f15a2f913fbde7e02d81fbefc99eff830deee223768decda541e45df6c907a", "48afc30652f7751483ad972d544eba3e1952b31721ec77f8b0d76dc08462db7d", "15f56e64494e54ed3da60434ff991a9c7a2e73b2fd691d714b34f843105e600e", "6eae98db08786d52c73fefb02e8bad7d89becf04421e7aa4f9ecc2ea58c0d79e")
	appendAcceptedGenerations(writes, "references/web-security/README.md", "2dcc85c7dab6afb2b7c80cd095e0ddf36f9b71565f89857263cb139aa099b6c8")
	appendAcceptedGenerations(writes, "references/web-security/agent-team.md", "1420a78fcb724e2d0129628b1510939a5e671b3683d40aa9c6e557fccda248db")
	appendAcceptedGenerations(writes, "references/web-security/workflow-template.md", "329ec012326c067c63b5e42c0ae2922b1ddf1e2d194b24e229070a3dbe631067")
	appendAcceptedGenerations(writes, "references/web-security/toolchain-router.md", "f4af42cdee9b1231a55a858632e468d6fe9bc8179be01a59b93f2cf0b068f86d")
	return writes
}

func appendAcceptedGenerations(writes map[string]PackRecoveryWrite, path string, sums ...string) {
	contract := writes[path]
	for _, sum := range sums {
		contract.AcceptedSHA256s[sum] = struct{}{}
	}
	writes[path] = contract
}

func packSkeletonWithCanonical(pack, readme, agent, workflow, router, handoff, canonicalReadme, canonicalWorkflow, canonicalHandoff string) map[string]PackRecoveryWrite {
	readmeGenerations := []string{readme}
	if canonicalReadme != "" {
		readmeGenerations = append(readmeGenerations, canonicalReadme)
	}
	readmeGenerations = append(readmeGenerations, repositoryIdentityReadmeGenerations[pack]...)
	prefix := "references/" + pack + "/"
	return map[string]PackRecoveryWrite{
		prefix + "README.md":            generation("managed-file", readmeGenerations...),
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
