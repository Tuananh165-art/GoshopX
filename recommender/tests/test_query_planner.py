import unittest

from app.chat.query_planner import QueryPlan, build_catalog_context, parse_query_plan


class QueryPlannerTests(unittest.TestCase):
    def test_extracts_budget_and_terms_without_executing_raw_filter(self):
        plan = parse_query_plan("Tìm laptop Dell dưới 20 triệu màu đen")

        self.assertIsInstance(plan, QueryPlan)
        self.assertEqual(plan.max_price, 20_000_000)
        self.assertEqual(plan.brand, "Dell")
        self.assertIn("laptop", plan.keywords)
        self.assertIn("màu đen", plan.keywords)
        self.assertNotIn("$", plan.filter_expression)

    def test_llm_plan_is_allow_listed(self):
        plan = parse_query_plan(
            "điện thoại gaming",
            model_plan={
                "category": "smartphones",
                "brand": "Acme",
                "min_price": -1,
                "max_price": "not-a-number",
                "filter_expression": "drop collection products",
            },
        )

        self.assertEqual(plan.category, "smartphones")
        self.assertEqual(plan.brand, "Acme")
        self.assertIsNone(plan.min_price)
        self.assertIsNone(plan.max_price)
        self.assertNotIn("drop", plan.filter_expression.lower())

    def test_catalog_context_is_bounded_and_contains_only_allowed_fields(self):
        context = build_catalog_context(
            [
                {
                    "id": "p1",
                    "name": "Phone",
                    "description": "Good",
                    "price": 100,
                    "thumbnail": "https://cdn.example/p1.jpg",
                    "api_key": "must-not-leak",
                }
            ]
        )

        self.assertIn("p1", context)
        self.assertIn("Phone", context)
        self.assertNotIn("must-not-leak", context)
        self.assertNotIn("api_key", context)


if __name__ == "__main__":
    unittest.main()
