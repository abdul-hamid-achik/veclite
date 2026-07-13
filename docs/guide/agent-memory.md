---
description: "Build caller-controlled agent memory with VecLite using TTL, importance, decay, conversations, subscriptions, consolidation, episodes, knowledge graphs, and eviction."
---

# Build Agent Memory

VecLite gives an agent durable memory records and retrieval primitives. It does not decide what the agent should remember, generate summaries or relationships, assign importance, or run a memory lifecycle on its own. Your application remains the policy layer.

| Policy owned by your application | VecLite primitive |
|---|---|
| Extract, redact, chunk, and embed a memory | `InsertWithOptions`, `InsertTurn`, `Embedder` |
| Decide relevance, scope, freshness, and importance | search options and filters |
| Decide when a memory expires | `WithTTL`, `WithExpiresAt`, `CleanupExpired` |
| Decide when and how memories are summarized | `FindSimilarClusters`, `Consolidate` |
| Decide what forms an experience | `CreateEpisode`, `DetectEpisodes` |
| Extract entities and relationships | `KnowledgeGraph` storage and traversal |
| Decide what can be forgotten | `WithMemoryLimits`, `EnforceMemoryLimit` |

The only background work in this guide is work you explicitly start, such as the TTL cleaner or memory limiter. Store provider credentials, prompt logic, safety rules, and rebuild policy in your application.

## Start with one memory collection

Use one collection when memories share an embedding profile. Enable BM25 if the agent also needs to recall exact names, IDs, commands, or quoted terms:

```go
db, err := veclite.Open("agent-memory.veclite", veclite.WithWAL(true))
if err != nil {
    log.Fatal(err)
}
defer func() { _ = db.Close() }()

memories, err := db.CreateCollection("memories",
    veclite.WithDimension(768),
    veclite.WithDistanceType(veclite.DistanceCosine),
    veclite.WithHNSW(16, 200),
    veclite.WithTextIndex("kind", "session", "source"),
)
if err != nil {
    log.Fatal(err)
}
```

If you configure `WithEmbedder`, helpers such as `InsertTurn` can embed text when you omit a vector. Otherwise, pass vectors explicitly. VecLite does not perform application-specific chunking or extraction.

## Store TTL and importance

Record lifecycle fields are opt-in:

```go
id, err := memories.InsertWithOptions(
    memoryVector,
    map[string]any{
        "kind":    "preference",
        "user_id": "user-42",
        "source":  "conversation",
    },
    veclite.WithContentOption("The user prefers concise status updates."),
    veclite.WithTTL(30*24*time.Hour),
    veclite.WithImportance(0.85),
)
```

`WithTTL` computes `ExpiresAt` from insertion time. `WithExpiresAt` sets an absolute time and wins if both options are present. `WithImportance` clamps values to the range `0`–`1`.

TTL is metadata until you act on it. An expired record remains stored and can still appear in search, sessions, and episode expansion. Hide it during retrieval:

```go
results, err := memories.Search(queryVector,
    veclite.TopK(10),
    veclite.WithFilter(veclite.NotExpired()),
)
```

Delete expired records on demand:

```go
deleted, err := memories.CleanupExpired()
```

Or explicitly start a database-wide cleaner. It runs one cleanup immediately and then on the interval:

```go
stopTTL := db.StartTTLCleaner(time.Minute, func(collection string, deleted int) {
    log.Printf("expired memory cleanup: collection=%s deleted=%d", collection, deleted)
})
defer stopTTL()
```

Use a positive interval. Closing the database also stops cleaners registered with it.

## Rank by relevance, recency, and importance

Importance does not change ordinary search by itself. Ask default-space vector `Search` to apply a boost, decay, or both:

```go
results, err := memories.Search(queryVector,
    // Retrieve a wider vector candidate set, rerank it, then return 10.
    veclite.TopK(100),
    veclite.WithLimit(10),
    veclite.WithFilters(
        veclite.NotExpired(),
        veclite.Not(veclite.Equal(veclite.PayloadKeyArchived, true)),
    ),
    veclite.WithDecay(veclite.DecayExponential, 7*24*time.Hour),
    veclite.WithImportanceBoost(0.5),
)
```

For exponential decay, a record's base score is multiplied by `2^(-age/halfLife)`. Linear decay reaches zero at the supplied duration; Gaussian decay treats the duration as sigma. Importance multiplies the score by `1 + boost × importance`.

These modifiers rerank the candidates already selected by vector similarity; they do not scan beyond `TopK`. Fetch more candidates and use `WithLimit` when a later rerank may change the order. `Threshold` is evaluated before the modifiers.

These modifiers are multiplicative rather than metric-aware. They are most useful for nonnegative, higher-is-better candidates—typically cosine retrieval with a positive threshold. With Euclidean distances or negative similarity scores, rerank in the application instead.

## Track conversations and threads

`InsertTurn` stores session, role, turn, and thread links as reserved payload fields:

```go
rootID, err := memories.InsertTurn(veclite.ConversationTurn{
    SessionID:  "session-202",
    TurnNumber: 1,
    Role:       "user",
    Content:    "Remember that the staging region is eu-west-1.",
    Vector:     userTurnVector,
    Importance: 0.8,
    TTL:        14 * 24 * time.Hour,
})
if err != nil {
    log.Fatal(err)
}

replyID, err := memories.InsertTurn(veclite.ConversationTurn{
    SessionID:     "session-202",
    TurnNumber:    2,
    Role:          "assistant",
    Content:       "I will use eu-west-1 for staging operations.",
    Vector:        assistantTurnVector,
    ParentChunkID: rootID,
})
if err != nil {
    log.Fatal(err)
}

session, _ := memories.GetSession("session-202")
thread, _ := memories.GetThread(replyID)
matches, _ := memories.SearchInSession(
    "session-202",
    queryVector,
    veclite.TopK(5),
    veclite.WithFilter(veclite.NotExpired()),
)

fmt.Println(len(session), len(thread), len(matches))
```

Your application assigns session IDs and turn numbers. `GetSession` sorts by turn number, `GetThread` follows parent/child links and sorts by creation time, and `SearchInSession` is vector search with a session filter. These methods do not automatically remove or hide expired turns.

If `ConversationTurn.Vector` is empty, `InsertTurn` uses the collection's configured embedder for non-empty content. Without a vector or embedder, it returns an error.

## Subscribe to future matching memories

Subscriptions are in-process signals for newly inserted default-space vectors:

```go
sub, err := memories.Subscribe(
    watchVector,
    veclite.WithSubscriptionThreshold(0.82),
    veclite.WithSubscriptionFilter(veclite.Equal("user_id", "user-42")),
    veclite.WithSubscriptionBufferSize(256),
)
if err != nil {
    log.Fatal(err)
}
defer func() { _ = memories.Unsubscribe(sub.ID) }()

for {
    select {
    case event, ok := <-sub.Events():
        if !ok {
            return
        }
        handleMatch(event.Record, event.Score)
    case <-ctx.Done():
        return
    }
}
```

Subscriptions are not durable queues:

- They are not persisted and do not replay existing records after creation or process restart.
- They do not emit update or delete events.
- They match only the default vector space; text-only and named-space-only records do not match.
- `Insert`, `InsertWithOptions`, `InsertRecord`, and helpers such as `InsertTurn` notify subscribers. `InsertDocument` and batch insertion are not currently event sources.
- Delivery is non-blocking. An event is dropped when the subscription buffer is full.
- Always set a metric-appropriate threshold. The default `0` accepts only nonnegative cosine/dot scores and only exact (`<= 0`) Euclidean matches.

Use `Unsubscribe` when finished so the collection removes the registration and closes its event channel.

## Consolidate related memories

Consolidation has two separate steps: VecLite finds clusters; your application decides how to summarize them.

Inspect candidate clusters without changing data:

```go
clusters, err := memories.FindSimilarClusters(veclite.ConsolidationConfig{
    SimilarityThreshold: 0.88,
    MinGroupSize:        3,
    MaxGroupSize:        12,
    Filters: []veclite.Filter{
        veclite.NotExpired(),
        veclite.Equal("user_id", "user-42"),
    },
})
```

To create summary records, provide both application-owned callbacks:

```go
result, err := memories.Consolidate(veclite.ConsolidationConfig{
    SimilarityThreshold: 0.88,
    MinGroupSize:        3,
    MaxGroupSize:        12,
    Filters:             []veclite.Filter{veclite.NotExpired()},

    // Your code calls a model, applies safety rules, and returns summary text.
    SummaryGenerator: summarizeMemories,

    // Your implementation of veclite.Embedder embeds that summary.
    Embedder: embedder,

    ArchiveOriginals: true,
})
```

`SummaryGenerator` has this signature:

```go
func(records []*veclite.Record) (summary string, payload map[string]any, err error)
```

Clustering uses the collection's default vectors and single-linkage comparisons, which cost roughly O(n²) over eligible records. It skips archived and previously consolidated records. Set filters to keep each run bounded by user, workspace, topic, or time window.

`ArchiveOriginals` does not delete the source records. It sets `PayloadKeyArchived`, and consolidated records retain their source IDs for `ExpandConsolidation`. Ordinary search does **not** hide archived records automatically, so keep the archival filter in your retrieval policy:

```go
notArchived := veclite.Not(veclite.Equal(veclite.PayloadKeyArchived, true))
```

VecLite does not schedule consolidation or choose a summarization model. Run it from your job system after measuring quality and cost.

## Group memories into episodes

Episodes add experience-level context without copying the underlying memory records:

```go
episodeStore, err := db.CreateEpisodeStore("memories")
if err != nil {
    log.Fatal(err)
}

detected, err := episodeStore.DetectEpisodes(veclite.EpisodeConfig{
    TimeGapThreshold: 20 * time.Minute,
    MinRecords:       2,
    MaxRecords:       50,
    Filters: []veclite.Filter{
        veclite.NotExpired(),
        veclite.Equal("user_id", "user-42"),
    },
})
if err != nil {
    log.Fatal(err)
}

contextual, err := episodeStore.SearchWithEpisodeExpansion(
    queryVector,
    veclite.TopK(5),
    veclite.WithFilter(veclite.NotExpired()),
)

fmt.Println(len(detected), len(contextual))
```

You can also define an episode explicitly:

```go
episode, err := episodeStore.CreateEpisode(
    []uint64{arrivalID, meetingID, followupID},
    "Customer onboarding call",
)
```

An episode stores record IDs, a time range, and the centroid of its members' default-space vectors. `DeleteEpisode` removes only the episode; it does not delete memories. If a referenced memory is later cleaned up or evicted, expansion skips the missing record.

Current episode detection is temporal: it groups adjacent records while their time gap remains within `TimeGapThreshold`. Although `EpisodeConfig` exposes `SimilarityThreshold`, the detector does not currently apply that field. Use filters, manual `CreateEpisode`, or your own pre-grouping when semantic coherence is required. Re-running detection creates additional episodes rather than reconciling previous runs, so your scheduler should own deduplication or replacement.

## Add a knowledge graph

The knowledge graph stores entities and relationships you have already extracted. VecLite validates IDs, persists the graph, indexes supplied entity vectors, and supports traversal plus vector-seeded expansion.

```go
graph, err := db.CreateKnowledgeGraph("assistant")
if err != nil {
    log.Fatal(err)
}

err = graph.AddEntity(veclite.Entity{
    ID:         "service:checkout",
    Type:       "service",
    Name:       "Checkout API",
    Vector:     checkoutVector,
    Properties: map[string]any{"team": "payments"},
})
if err != nil {
    log.Fatal(err)
}

err = graph.AddEntity(veclite.Entity{
    ID:     "runbook:pool-exhaustion",
    Type:   "runbook",
    Name:   "Pool exhaustion runbook",
    Vector: runbookVector,
})
if err != nil {
    log.Fatal(err)
}

err = graph.AddRelationship(veclite.Relationship{
    ID:        "checkout-has-runbook",
    SourceID:  "service:checkout",
    TargetID:  "runbook:pool-exhaustion",
    Type:      "has_runbook",
    Weight:    0.95,
})
if err != nil {
    log.Fatal(err)
}

expanded, err := graph.SearchWithExpansion(
    queryVector,
    veclite.TraversalConfig{
        MaxDepth:          1,
        MaxNodes:          20,
        MinWeight:         0.5,
        RelationshipTypes: []string{"has_runbook"},
        Direction:         "both",
    },
    veclite.TopK(5),
)
```

`SearchWithExpansion` vector-searches entities, then adds immediate graph neighbors when `MaxDepth` is positive. It currently expands one hop even if you pass a larger depth. For multi-hop context, call `Traverse` explicitly:

```go
context, err := graph.Traverse(
    []string{"service:checkout"},
    veclite.TraversalConfig{MaxDepth: 3, MaxNodes: 100, Direction: "outgoing"},
)
```

Your application creates entity IDs, embeddings, relationship types, weights, and extraction rules. The graph does not infer entities or relationships from memory content.

## Bound memory with eviction

Configure a record-count limit when creating the collection:

```go
memories, err := db.CreateCollection("memories",
    veclite.WithDimension(768),
    veclite.WithMemoryLimits(veclite.MemoryConfig{
        MaxRecords:        50_000,
        EvictionPolicy:    "lru",
        EvictionBatchSize: 500,
        CleanupInterval:   time.Minute,
    }),
)
```

Available policies are:

| Policy | Evicts first | Requirement |
|---|---|---|
| `fifo` | Oldest `CreatedAt` | None; this is also the fallback for an unknown policy. |
| `lru` | Never-accessed, then oldest `LastAccessedAt` | Retrieve with `WithAccessTracking(true)`. |
| `importance` | Lowest `Importance` | Assign importance on insertion. |

`WithMemoryLimits` enforces the limit after supported inserts. A positive `CleanupInterval` also starts a background check. For an existing collection, start a limiter explicitly or invoke it on demand:

```go
_, err = memories.Search(queryVector,
    veclite.TopK(10),
    veclite.WithAccessTracking(true),
)

evicted := memories.EnforceMemoryLimit(veclite.MemoryConfig{
    MaxRecords:        50_000,
    EvictionPolicy:    "lru",
    EvictionBatchSize: 500,
})
```

Eviction is destructive. It counts records rather than bytes and removes at most one batch per enforcement pass. Archived records are protected, so enough archived records can keep the collection above `MaxRecords`. Coordinate eviction with episodes and consolidations that retain record IDs.

Automatic eviction is currently safest for memories that use only the default vector space. The limiter removes default-space and BM25 index entries but does not remove named-space HNSW nodes. For named-space collections, implement a caller-owned selection policy and delete chosen records with `Collection.Delete`, which cleans every space.

Access tracking is also opt-in. Without `WithAccessTracking(true)`, LRU treats untouched records as never accessed. Access bookkeeping is included in full snapshots but is not itself appended to the WAL, so do not treat it as a crash-durable audit log.

## Know the retrieval limitations

Memory options are not applied uniformly across every search API:

| Retrieval path | Decay / importance boost | Access tracking | Expiry / archive behavior |
|---|---|---|---|
| Default-space `Search` | Applied when requested | Applied when requested | Only through explicit filters |
| `TextSearch` | Not applied | Not applied | Only through explicit filters |
| `HybridSearch` / `HybridSearchSpace` | Not applied | Not applied | Filters apply to both legs |
| Direct `SearchSpace` | Not applied | Applied when requested | Only through explicit filters |
| `MultiSpaceSearch` | Not applied | Not applied | Only through explicit filters |

Hybrid results use RRF scores, not vector similarity. `Threshold` filters only the vector leg of hybrid search; it does not threshold BM25 or the final fused ranking. `WithVectorWeight` and `WithTextWeight` change rank contributions but do not calibrate relevance, recency, or importance.

For a caller-controlled hybrid memory policy, fetch broad lists, fuse them, and rerank the fused candidates in your application:

```go
vectorResults, _ := memories.Search(queryVector,
    veclite.TopK(100),
    veclite.WithFilter(veclite.NotExpired()),
)
textResults, _ := memories.TextSearch(queryText,
    veclite.TopK(100),
    veclite.WithFilter(veclite.NotExpired()),
)

fused := veclite.FuseRRF(
    [][]veclite.Result{vectorResults, textResults},
    veclite.WithFusionWeights(1.0, 0.7),
    veclite.WithFusionTopK(100),
)

// Your application can now rerank fused using Record.CreatedAt,
// Record.Importance, authorization rules, and a model-based reranker.
final := rerankMemories(fused)
if len(final) > 10 {
    final = final[:10]
}
```

## What persists

Records, TTLs, importance, conversation payloads, consolidation records, episode stores, and knowledge graphs are part of database persistence. Subscriptions, callbacks, running cleaners/limiters, summarizer implementations, and schedules are process-local and must be recreated after restart.

For durability choices, see [Durability and Multi-Process Access](./durability.md). For retrieval semantics and score calibration, see [Choose a Search Strategy](./search.md).
