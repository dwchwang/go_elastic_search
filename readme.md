# Elastic Product Demo

Demo REST API quản lý sản phẩm bằng **Go + Gin + Elasticsearch**, tập trung thực hành các tính năng quan trọng của Elasticsearch. Chỉ dùng 1 index: `products`.

---

## Tech Stack

- **Go** 1.25.3 · **Gin** v1.12.0
- **Elasticsearch** 8.15.3 · **Kibana** 8.15.3
- `github.com/elastic/go-elasticsearch/v8` v8.19.5
- Docker Compose

---

## Cấu trúc project

```
cmd/api/main.go                   # Entry point, Gin router
internal/
  esclient/client.go              # Khởi tạo và kiểm tra kết nối ES
  product/
    module.go                     # Đăng ký routes
    model.go                      # Struct: Product, ProductSpec, response types
    helpers.go                    # Hàm tiện ích dùng chung
    index.go                      # Tạo index với explicit mapping + custom analyzer
    seed.go                       # Bulk import từ NDJSON
    crud.go                       # Create / Get / Update / Delete
    search.go                     # Full-text search, nested spec search
    update.go                     # Scripted update, upsert, concurrency control
    analytics.go                  # Aggregation analytics
data/products.ndjson              # Dữ liệu mẫu
docker-compose.yml
```



---

## API chức năng chính

| Method | Path | Mô tả |
|---|---|---|
| GET | `/health` | Kiểm tra kết nối |
| POST | `/setup` | Tạo index với mapping |
| POST | `/seed` | Bulk import từ NDJSON |
| POST | `/products` | Tạo sản phẩm |
| GET | `/products/:id` | Lấy sản phẩm (kèm `_seq_no`, `_primary_term`) |
| PATCH | `/products/:id` | Cập nhật một phần |
| DELETE | `/products/:id` | Xóa sản phẩm |
| GET | `/search/products` | Full-text search với filter, sort, phân trang, highlight |
| GET | `/search/products/specs` | Tìm theo nested spec (`?name=storage&value=256GB`) |
| POST | `/products/:id/decrease-stock` | Giảm stock bằng Painless script |
| POST | `/products/:id/view` | Tăng view_count (upsert nếu chưa tồn tại) |
| PATCH | `/products/:id/concurrent` | Cập nhật có optimistic concurrency control |
| GET | `/analytics/products` | Thống kê tổng hợp bằng aggregation |

**Query params của `/search/products`:** `q`, `category`, `brand`, `min_price`, `max_price`, `sort` (`price_asc` / `price_desc` / `rating_desc` / `newest`), `page`, `size` (max 50).

**Query params của `/products/:id/concurrent`:** `seq_no`, `primary_term` lấy từ GET trước đó.

---

## Kiến thức Elasticsearch áp dụng

### Index Mapping & Field Types
- `text` để full-text search, `keyword` cho filter/exact/aggregation.
- **Multi-field**: `name.keyword` vừa search được vừa sort chính xác.
- **`nested`**: field `specs` dùng kiểu nested để ES không flatten mất mối quan hệ `name`/`value`.

### Custom Analyzer & Synonym
- `product_search_analyzer`: lowercase + synonym (`laptop↔notebook`, `phone↔smartphone`, …).
- `english_text_analyzer`: lowercase + stop words + Porter stemmer cho `description`.

### CRUD Document
- Index API với `WithDocumentID`, Update API với `{"doc": {...}}`, Delete API.
- Get trả thêm `_seq_no` và `_primary_term` phục vụ concurrency control.

### Bulk API
- Import nhiều document trong một request từ file NDJSON.
- Response HTTP 200 không đồng nghĩa tất cả item thành công — phải kiểm tra field `errors` trong body.

### Full-text Search
- `multi_match` trên `name^3` + `description` với `fuzziness: AUTO` (chịu lỗi chính tả).
- `bool.must` ảnh hưởng score; `bool.filter` (term, range) không ảnh hưởng score, nhanh hơn.
- Highlight kết quả khớp với thẻ `<mark>`.
- Sort theo field, phân trang bằng `from` + `size`.

### Nested Query & Inner Hits
- `nested` query đảm bảo cặp `specs.name` + `specs.value` khớp cùng một object, không cross-match.
- `inner_hits` trả về chính xác những nested object nào đã khớp.

### Scripted Update (Painless)
- Thực thi logic nguyên tử trực tiếp trong ES: giảm stock, kiểm tra điều kiện.
- `ctx.op = 'noop'` để bỏ qua update khi điều kiện không thỏa mãn (stock không đủ).

### Upsert
- Nếu document tồn tại: chạy script tăng `view_count`.
- Nếu chưa tồn tại: tạo mới với giá trị mặc định trong `upsert`.

### Optimistic Concurrency Control
- Gửi kèm `if_seq_no` + `if_primary_term` khi update.
- Document đã bị thay đổi bởi request khác → ES trả `409 Conflict`.
- Tránh lost-update mà không cần lock.

### Aggregation
- **Metric**: `stats` (min/max/avg/sum/count), `cardinality` (unique count), `percentiles`.
- **Bucket**: `terms` (group by), `range` (chia khoảng giá).
- **Sub-aggregation**: `avg`, `min`, `max`, `sum` lồng trong `terms` bucket.
- `"size": 0` để chỉ trả aggregation, không trả document hits.

---

## Data Model

```json
{
  "id": "prod-001",
  "name": "MacBook Pro 14 M3",
  "brand": "Apple",
  "category": "laptop",
  "price": 49990000,
  "stock": 15,
  "rating": 4.8,
  "view_count": 120,
  "tags": ["laptop", "apple", "m3"],
  "specs": [
    { "name": "cpu", "value": "Apple M3" },
    { "name": "storage", "value": "512GB" }
  ],
  "created_at": "2024-10-01"
}
```
