# Structured spatial questions

`Build(scene, types, conditions, language)` chooses a randomized applicable category and tries at most 64 expressions per category. An empty type list enables all eight categories. Difficulty is exact: one atomic condition for easy, two for medium, three for hard. No category is silently substituted when an explicit list is supplied. Unsuitable scenes produce an error, allowing the caller to generate a different scene.

Before searching, Build independently samples a desired answer uniformly from 1 through the scene's shape count minus one. It accepts only candidates with that answer, rather than preferentially returning the first singleton expression. Some sampled answers are infeasible, so callers must handle bounded failure. Successful output is still conditioned on scene/type feasibility: this is not a guarantee of uniform answers or equal category frequencies. Application-level security must not rely on those distributions being uniform.

`Question.Expr`, not question text or image metadata, determines `Answer`. `Count` evaluates the expression against every scene shape. `Validate` additionally checks readable geometry, the stored answer, an exact English or Chinese AST rendering, meaningful logical branches, and a nonempty proper subset of the scene. Neither operation reads cached visibility metrics. `Count` is deliberately a lower-level operation: call `Validate` before exposing a custom question.

## AST extension

Implement `Predicate` with deterministic `Evaluate`, bilingual `Describe`, and `Conditions` methods. Evaluation must use scene geometry only, must not mutate the scene, and must return an error for ambiguous boundaries. A custom expression can be combined with `And`, `Or`, and `Not`; callers can use `Render` and `Count` to construct a `Question`. Built-in logical nodes evaluate every child rather than allowing short-circuiting to hide an ambiguous boundary. Custom implementations remain responsible for validating their own semantic inputs.

Available leaves are `ColorIs`, `KindIs`, `Position`, `Related`, `OcclusionStatus`, and `CountComparison`. `Related` uses actual geometry through `scene.Relate`. `Position` explicitly describes shape centers and rejects the closed band within 3% of the canvas dimension around its midline. `CountComparison` counts other shapes selected by its predicate and supports equal, at-least, at-most, greater, and less comparisons. Its selected target set must be nonempty for each candidate.

`Reference` selects by color, kind, or both, never IDs or labels. A normal reference must resolve to exactly one shape. `All: true` explicitly renders universal quantification and requires a nonempty set. A candidate is not related to itself, so a target member cannot satisfy a universal relation over its own set. Generated multi-target questions require at least two reference matches. A conjunction must change its selected set when any branch is removed; disjunction branches must likewise contribute results. This prevents redundant filters from pretending to increase difficulty.

## Category suitability

`Types` lists the eight categories; `Constraints` exposes their allowed condition range and requirements programmatically.

| Category | Conditions | Required scene properties |
| --- | --- | --- |
| Basic | 1 | Color or kind selects a nontrivial subset. |
| Spatial | 1–3 | Clear canvas half or spatial relation to a unique descriptor. |
| ColorShape | 2–3 | Color and kind each meaningfully narrow the result. |
| Occlusion | 1–3 | Readable partial occlusion or its absence; reference-based variants require a unique occluder. |
| Size | 1–3 | Clear analytic-area relation to a unique descriptor. |
| Compound | 2–3 | Meaningful explicit conjunction/disjunction or relational count comparison. |
| MultiTarget | 1–3 | At least two targets share a descriptor and a universal size/spatial relation is nontrivial. |
| Exclusion | 1–3 | Explicit negation selects a nontrivial subset. |

Single-color or single-kind configurations are supported where their remaining distinctions make a requested type meaningful. A single-color/single-kind scene cannot support a basic attribute count, but can support positional or suitable relational questions. Geometry ambiguity thresholds and minimum visibility are documented by `scene.Relate` and `scene.Validate`. Near/far, area, containment, and occlusion are never inferred from arbitrary words or bounding-box overlap.
