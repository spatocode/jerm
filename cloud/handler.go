package cloud

const (
	awsLambdaHandler = `
import json
import io
import logging
import traceback

from wsgi_adapter import LambdaWSGIHandler
from django.core import management
from django.conf import settings

from .wsgi import application


logging.basicConfig()
logger = logging.getLogger()
logger.setLevel(logging.INFO)


def lambda_handler(event, context):
    if settings.DEBUG:
        logger.debug("Bulaba Event: {}".format(event))

    if event.get("manage"):
        output = io.StringIO()
        management.call_command(*event["manage"].split(" "), stdout=output)
        return {"output": output.getvalue()}

    try:
        if event.get("httpMethod", None):
            handler = LambdaWSGIHandler(application)
            return handler(event, context)
    except Exception as e:
        print(e)
        exc_info = sys.exc_info()
        message = ("An unexpected error occured during this request. Run 'bulaba logs' to inspect logs.")
        content = {
            "body": json.dumps(str({"message": message})),
            "statusCode": 500
        }
        if settings.DEBUG:
            content["traceback"] = traceback.format_exception(*exc_info)
        return content
	`
)
