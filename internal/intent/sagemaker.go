package intent

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// sageMakerInstanceTypeRE validates SageMaker instance type strings (e.g. ml.g5.2xlarge).
var sageMakerInstanceTypeRE = regexp.MustCompile(`^ml\.[a-z][0-9a-z]+\.[a-z0-9]+$`)

// smInstanceGPUCount is a static table mapping known SageMaker g5/g6 instance types
// to their GPU count. Used by the GPU-sizing preflight in validateSageMaker.
//
// GPU counts (A10G for g5, L4 for g6 — both 24 GiB per GPU):
//   - xlarge / 2xlarge / 4xlarge / 8xlarge / 16xlarge: 1 GPU
//   - 12xlarge / 24xlarge: 4 GPUs
//   - 48xlarge: 8 GPUs
//
// Unknown instance types are NOT in this table. The sizing rule fails-open for
// any instance type not present here (skip the GPU check — no error, no panic).
// This mirrors the gpuInstanceAZDeny known-gaps idiom used in cluster.go.
var smInstanceGPUCount = map[string]int{
	// g5 family (NVIDIA A10G, 24 GiB each)
	"ml.g5.xlarge":   1,
	"ml.g5.2xlarge":  1,
	"ml.g5.4xlarge":  1,
	"ml.g5.8xlarge":  1,
	"ml.g5.16xlarge": 1,
	"ml.g5.12xlarge": 4,
	"ml.g5.24xlarge": 4,
	"ml.g5.48xlarge": 8,
	// g6 family (NVIDIA L4, 24 GiB each — same GPU count ladder as g5)
	"ml.g6.xlarge":   1,
	"ml.g6.2xlarge":  1,
	"ml.g6.4xlarge":  1,
	"ml.g6.8xlarge":  1,
	"ml.g6.16xlarge": 1,
	"ml.g6.12xlarge": 4,
	"ml.g6.24xlarge": 4,
	"ml.g6.48xlarge": 8,
}

// DefaultSyntheticImage is the default container image used for simulated vLLM inference.
const DefaultSyntheticImage = "ghcr.io/llm-d/llm-d-inference-sim:v0.11.2"

// AISpec is the top-level opt-in AI inference block in cluster.yaml.
// When absent (nil) or Enabled=false, all AI-related phases are skipped;
// existing cluster.yaml files that omit this block are completely unaffected.
//
// Example:
//
//	ai:
//	  sagemaker:
//	    enabled: true
//	    model: meta-llama/Meta-Llama-3-8B-Instruct
//	    instanceType: ml.g5.2xlarge
//	    scaleToZero: false
//	  synthetic:
//	    enabled: true
//	    profile: llama-3.3-70b
type AISpec struct {
	SageMaker *SageMakerSpec `yaml:"sagemaker,omitempty"`
	Synthetic *SyntheticSpec `yaml:"synthetic,omitempty"`
}

// SyntheticSpec configures the lightweight synthetic/simulated vLLM endpoint
// using llm-d-inference-sim. Runs on standard CPU nodes without requiring
// physical GPUs, HuggingFace tokens, or model weight downloads.
//
// Profile picks the model size to simulate (SyntheticProfiles); any field set
// here overrides the profile's value.
type SyntheticSpec struct {
	// Enabled is the master switch for synthetic inference simulation.
	Enabled bool `yaml:"enabled"`
	// Image is the container image to run (default: DefaultSyntheticImage).
	Image string `yaml:"image,omitempty"`
	// Profile is the simulated model size (default: DefaultSyntheticProfile).
	Profile string `yaml:"profile,omitempty"`
	// Model is the Hugging Face repo the simulator serves and tokenizes with; endpoint
	// pickers match its KV events against it (default: the profile's ungated repo).
	Model string `yaml:"model,omitempty"`
	// ServedModelName is an alias the simulator also answers to (default: "llama3").
	ServedModelName string `yaml:"servedModelName,omitempty"`
	// Replicas is the number of simulator pods (default: 1).
	Replicas int `yaml:"replicas,omitempty"`
	// TTFTBaseMs is the fixed prefill overhead in ms, before per-token prefill.
	TTFTBaseMs int `yaml:"ttftBaseMs,omitempty"`
	// PrefillUsPerToken is the prefill time per uncached prompt token in µs.
	PrefillUsPerToken int `yaml:"prefillUsPerToken,omitempty"`
	// ITLMs is the Inter-Token Latency per output token in ms.
	ITLMs float64 `yaml:"itlMs,omitempty"`
	// MaxNumSeqs is the number of requests served at once; the rest queue.
	MaxNumSeqs int `yaml:"maxNumSeqs,omitempty"`
	// TimeFactorUnderLoad is the slowdown when all MaxNumSeqs slots are busy (>= 1).
	TimeFactorUnderLoad float64 `yaml:"timeFactorUnderLoad,omitempty"`
	// MaxModelLen is the context window in tokens.
	MaxModelLen int `yaml:"maxModelLen,omitempty"`
	// KVCacheBlocks is the KV cache size in 64-token blocks.
	KVCacheBlocks int `yaml:"kvCacheBlocks,omitempty"`
}

// DefaultSyntheticProfile is the model size simulated when ai.synthetic.profile is unset.
const DefaultSyntheticProfile = "llama-3.1-8b"

// SyntheticProfiles holds the timing of each simulated model size. 8B and 70B
// follow NVIDIA's NIM benchmarks on H100 (FP8, docs.nvidia.com/nim/benchmarking/llm):
// TTFT at 200/5000/20000 input tokens gives the overhead and per-token prefill,
// ITL is the concurrency-1 value, and the load factor the ITL growth at max
// concurrency. 405B (TP8) is scaled from those, NVIDIA publishes no latency table.
// KV cache sizes are estimates of H100 memory left after weights.
var SyntheticProfiles = map[string]SyntheticSpec{
	"llama-3.1-8b": {
		Model: "NousResearch/Meta-Llama-3.1-8B-Instruct", TTFTBaseMs: 7, PrefillUsPerToken: 18, ITLMs: 4.6,
		MaxNumSeqs: 256, TimeFactorUnderLoad: 2.5, MaxModelLen: 131072, KVCacheBlocks: 7000,
	},
	"llama-3.3-70b": {
		Model: "unsloth/Llama-3.3-70B-Instruct", TTFTBaseMs: 14, PrefillUsPerToken: 45, ITLMs: 13.8,
		MaxNumSeqs: 256, TimeFactorUnderLoad: 2.0, MaxModelLen: 131072, KVCacheBlocks: 12000,
	},
	"llama-3.1-405b": {
		Model: "NousResearch/Hermes-3-Llama-3.1-405B", TTFTBaseMs: 30, PrefillUsPerToken: 120, ITLMs: 38,
		MaxNumSeqs: 128, TimeFactorUnderLoad: 2.0, MaxModelLen: 131072, KVCacheBlocks: 7000,
	},
}

// SageMakerSpec configures the disposable SageMaker LMI endpoint.
// When Enabled is false (or the ai.sagemaker block is omitted), no SageMaker
// resources are created. On up, three AWS resources are created in order:
// SageMaker Model → EndpointConfig → Endpoint. On down, they are deleted
// in reverse order so nothing bills between sessions.
//
// LMI container: DJL-Serving Large Model Inference (DJL 0.36.0 / LMI v25,
// serving engine IS vLLM 0.20.1). Model configuration is passed via
// environment variables (HF_MODEL_ID, ROLLING_BATCH=vllm).
// Instance default: ml.g5.2xlarge ($1.97/hr per spec).
type SageMakerSpec struct {
	// Enabled is the master switch. Default false — omitting the block or
	// setting enabled: false leaves the cluster unaffected.
	Enabled bool `yaml:"enabled"`
	// Model is the Hugging Face model ID, e.g. "meta-llama/Meta-Llama-3-8B-Instruct".
	// Required when Enabled is true.
	Model string `yaml:"model,omitempty"`
	// InstanceType for the SageMaker endpoint. Default "ml.g5.2xlarge".
	InstanceType string `yaml:"instanceType,omitempty"`
	// ScaleToZero configures a managed-instance-scaling endpoint-config with
	// MinInstanceCount=0 (scale to zero when idle). When false (default), the
	// endpoint keeps exactly one instance running.
	ScaleToZero bool `yaml:"scaleToZero,omitempty"`
	// ImageURI, when set, is used VERBATIM as the SageMaker model container image
	// (full URI including registry and tag), bypassing the default DLC construction.
	// Use this to pin a specific LMI DLC tag or substitute a private ECR image.
	// AWS periodically deprecates older DLC tags — set this field when the default
	// tag is no longer available rather than waiting for a code update.
	// Example: "763104351884.dkr.ecr.ap-southeast-2.amazonaws.com/djl-inference:0.36.0-lmi25.0.0-cu130"
	// When empty (default), the phase constructs the URI from the cluster region.
	ImageURI string `yaml:"imageUri,omitempty"`
	// TensorParallelSize sets OPTION_TENSOR_PARALLEL_DEGREE in the LMI vLLM
	// container environment. Zero (default) = unset, LMI uses its own default
	// (typically 1). For large models such as Qwen-32B on ml.g5.12xlarge, set
	// this to the number of GPUs in the instance (e.g. 4 for ml.g5.12xlarge).
	// Must be ≤ the GPU count of the chosen instanceType; validation fails fast
	// for known g5/g6 instance types. Max 8 (no current g5/g6 SKU exceeds 8 GPUs).
	TensorParallelSize int `yaml:"tensorParallelSize,omitempty"`
	// MaxModelLen sets OPTION_MAX_MODEL_LEN in the LMI vLLM container environment.
	// Zero (default) = unset, LMI uses vLLM's own default. Set to cap the KV
	// cache and reduce GPU memory pressure for large models (e.g. 8192 for Qwen-32B
	// on 4×A10G/96 GiB). Must be non-negative.
	MaxModelLen int `yaml:"maxModelLen,omitempty"`
}

// SageMakerEnabled reports whether the SageMaker endpoint should be
// provisioned. True only when ai.sagemaker.enabled is explicitly true.
func (c *Cluster) SageMakerEnabled() bool {
	return c.AI != nil && c.AI.SageMaker != nil && c.AI.SageMaker.Enabled
}

// validateSageMaker checks the ai.sagemaker block constraints.
// Called from validate() only when AI != nil && AI.SageMaker != nil && Enabled.
//
// Rules:
//   - model must be non-empty when enabled.
//   - instanceType must match instanceTypeRE when set.
//   - tensorParallelSize must be >= 0 and <= 8.
//   - maxModelLen must be >= 0.
//   - GPU-sizing preflight: if tensorParallelSize > 0 and instanceType is in the
//     known table, tensorParallelSize must be <= the instance's GPU count.
//     Unknown instance types are skipped (fail-open, no panic).
func validateSageMaker(s *SageMakerSpec) error {
	if !s.Enabled {
		return nil
	}
	if s.Model == "" {
		return fmt.Errorf("ai.sagemaker.model is required when ai.sagemaker.enabled is true")
	}
	if s.InstanceType != "" && !sageMakerInstanceTypeRE.MatchString(s.InstanceType) {
		return fmt.Errorf("ai.sagemaker.instanceType %q does not match expected pattern (e.g. ml.g5.2xlarge)", s.InstanceType)
	}
	if s.TensorParallelSize < 0 {
		return fmt.Errorf("ai.sagemaker.tensorParallelSize must be >= 0, got %d", s.TensorParallelSize)
	}
	if s.TensorParallelSize > 8 {
		return fmt.Errorf("ai.sagemaker.tensorParallelSize %d exceeds 8 (no current ml.g5/ml.g6 SKU has more than 8 GPUs)", s.TensorParallelSize)
	}
	if s.MaxModelLen < 0 {
		return fmt.Errorf("ai.sagemaker.maxModelLen must be >= 0, got %d", s.MaxModelLen)
	}
	// GPU-sizing preflight: fail fast when tensorParallelSize exceeds the instance's
	// physical GPU count. Fails-open for unknown instance types (not in the table).
	if s.TensorParallelSize > 0 && s.InstanceType != "" {
		if gpuCount, known := smInstanceGPUCount[s.InstanceType]; known {
			if s.TensorParallelSize > gpuCount {
				// Find an example instance with enough GPUs for the error message.
				example := ""
				for _, candidate := range []string{
					"ml.g5.12xlarge", "ml.g5.24xlarge", "ml.g5.48xlarge",
					"ml.g6.12xlarge", "ml.g6.24xlarge", "ml.g6.48xlarge",
				} {
					if n, ok := smInstanceGPUCount[candidate]; ok && n >= s.TensorParallelSize {
						example = candidate
						break
					}
				}
				hint := fmt.Sprintf("pick an instance with >= %d GPUs", s.TensorParallelSize)
				if example != "" {
					hint = fmt.Sprintf("pick an instance with >= %d GPUs (e.g. %s has %d) or lower tensorParallelSize",
						s.TensorParallelSize, example, smInstanceGPUCount[example])
				}
				return fmt.Errorf("ai.sagemaker.tensorParallelSize %d exceeds GPU count for %s (%d GPU); %s",
					s.TensorParallelSize, s.InstanceType, gpuCount, hint)
			}
		}
	}
	return nil
}

// applySageMakerDefaults fills zero-value SageMaker fields.
// Called from applyDefaults when ai.sagemaker is present.
func applySageMakerDefaults(s *SageMakerSpec) {
	if s.InstanceType == "" {
		s.InstanceType = "ml.g5.2xlarge"
	}
}

// SyntheticAIEnabled reports whether synthetic AI inference simulation is enabled.
func (c *Cluster) SyntheticAIEnabled() bool {
	return c.AI != nil && c.AI.Synthetic != nil && c.AI.Synthetic.Enabled
}

// UsesSyntheticAI reports whether the AI scenario runs the simulator: ai.synthetic is
// enabled, or the cluster has node groups and none of them has GPUs.
func (c *Cluster) UsesSyntheticAI() bool {
	if c == nil {
		return false
	}
	if c.SyntheticAIEnabled() {
		return true
	}
	if c.ClusterSpec == nil || len(c.ClusterSpec.NodeGroups) == 0 {
		return false
	}
	for _, ng := range c.ClusterSpec.NodeGroups {
		if ng.IsGPU() {
			return false
		}
	}
	return true
}

// SimulatorTags labels benchmark results with what the simulator stands in for
// (empty on a GPU cluster). Forge reads the same from the simulator pods.
func (c *Cluster) SimulatorTags() map[string]string {
	if !c.UsesSyntheticAI() {
		return nil
	}
	s := c.ResolvedSynthetic()
	return map[string]string{"model_server": "llm-d-inference-sim", "sim_profile": s.Profile, "sim_model": s.Model}
}

// ResolvedSynthetic returns ai.synthetic with every default applied, also when the block
// is absent (the AI scenario falls back to the simulator on clusters without GPUs).
func (c *Cluster) ResolvedSynthetic() SyntheticSpec {
	var s SyntheticSpec
	if c != nil && c.AI != nil && c.AI.Synthetic != nil {
		s = *c.AI.Synthetic
	}
	applySyntheticDefaults(&s)
	return s
}

// validateSynthetic checks the ai.synthetic block constraints.
func validateSynthetic(s *SyntheticSpec) error {
	if !s.Enabled {
		return nil
	}
	if s.Profile != "" {
		if _, ok := SyntheticProfiles[s.Profile]; !ok {
			return fmt.Errorf("ai.synthetic.profile %q is not one of %s", s.Profile, strings.Join(SyntheticProfileNames(), ", "))
		}
	}
	for name, v := range map[string]float64{
		"replicas": float64(s.Replicas), "ttftBaseMs": float64(s.TTFTBaseMs), "prefillUsPerToken": float64(s.PrefillUsPerToken),
		"itlMs": s.ITLMs, "maxNumSeqs": float64(s.MaxNumSeqs), "maxModelLen": float64(s.MaxModelLen), "kvCacheBlocks": float64(s.KVCacheBlocks),
	} {
		if v < 0 {
			return fmt.Errorf("ai.synthetic.%s must be >= 0, got %v", name, v)
		}
	}
	if s.TimeFactorUnderLoad != 0 && s.TimeFactorUnderLoad < 1 {
		return fmt.Errorf("ai.synthetic.timeFactorUnderLoad must be >= 1, got %v", s.TimeFactorUnderLoad)
	}
	return nil
}

// SyntheticProfileNames lists the known ai.synthetic.profile values, sorted.
func SyntheticProfileNames() []string {
	names := make([]string, 0, len(SyntheticProfiles))
	for n := range SyntheticProfiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// applySyntheticDefaults fills zero-value synthetic inference fields from the profile.
func applySyntheticDefaults(s *SyntheticSpec) {
	if s.Image == "" {
		s.Image = DefaultSyntheticImage
	}
	if s.Profile == "" {
		s.Profile = DefaultSyntheticProfile
	}
	p := SyntheticProfiles[s.Profile]
	if s.Model == "" {
		s.Model = p.Model
	}
	if s.ServedModelName == "" {
		s.ServedModelName = "llama3"
	}
	if s.Replicas <= 0 {
		s.Replicas = 1
	}
	if s.TTFTBaseMs <= 0 {
		s.TTFTBaseMs = p.TTFTBaseMs
	}
	if s.PrefillUsPerToken <= 0 {
		s.PrefillUsPerToken = p.PrefillUsPerToken
	}
	if s.ITLMs <= 0 {
		s.ITLMs = p.ITLMs
	}
	if s.MaxNumSeqs <= 0 {
		s.MaxNumSeqs = p.MaxNumSeqs
	}
	if s.TimeFactorUnderLoad <= 0 {
		s.TimeFactorUnderLoad = p.TimeFactorUnderLoad
	}
	if s.MaxModelLen <= 0 {
		s.MaxModelLen = p.MaxModelLen
	}
	if s.KVCacheBlocks <= 0 {
		s.KVCacheBlocks = p.KVCacheBlocks
	}
}
