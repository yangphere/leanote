`app/info` contains the shared domain models, API envelopes and projections.
Persistence-facing fields keep their explicit BSON tags, while identifier
fields use the framework-neutral `app/domain.ObjectID` type. BSON codecs and
Mongo client details belong to `app/db`; this package must remain independent
of Revel and the Mongo driver.
