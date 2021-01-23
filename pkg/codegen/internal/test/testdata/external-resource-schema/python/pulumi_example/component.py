# coding=utf-8
# *** WARNING: this file was generated by test. ***
# *** Do not edit by hand unless you're certain you know what you are doing! ***

import warnings
import pulumi
import pulumi.runtime
from typing import Any, Mapping, Optional, Sequence, Union
from . import _utilities, _tables
import pulumi_aws
import pulumi_kubernetes

__all__ = ['Component']


class Component(pulumi.CustomResource):
    def __init__(__self__,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None,
                 metadata: Optional[pulumi.Input[pulumi.InputType['pulumi_kubernetes.meta.v1.ObjectMetaArgs']]] = None,
                 __props__=None,
                 __name__=None,
                 __opts__=None):
        """
        Create a Component resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param pulumi.ResourceOptions opts: Options for the resource.
        """
        if __name__ is not None:
            warnings.warn("explicit use of __name__ is deprecated", DeprecationWarning)
            resource_name = __name__
        if __opts__ is not None:
            warnings.warn("explicit use of __opts__ is deprecated, use 'opts' instead", DeprecationWarning)
            opts = __opts__
        if opts is None:
            opts = pulumi.ResourceOptions()
        if not isinstance(opts, pulumi.ResourceOptions):
            raise TypeError('Expected resource options to be a ResourceOptions instance')
        if opts.version is None:
            opts.version = _utilities.get_version()
        if opts.id is None:
            if __props__ is not None:
                raise TypeError('__props__ is only valid when passed in combination with a valid opts.id to get an existing resource')
            __props__ = dict()

            __props__['metadata'] = metadata
            __props__['provider'] = None
            __props__['security_group'] = None
            __props__['storage_classes'] = None
        super(Component, __self__).__init__(
            'example::Component',
            resource_name,
            __props__,
            opts)

    @staticmethod
    def get(resource_name: str,
            id: pulumi.Input[str],
            opts: Optional[pulumi.ResourceOptions] = None) -> 'Component':
        """
        Get an existing Component resource's state with the given name, id, and optional extra
        properties used to qualify the lookup.

        :param str resource_name: The unique name of the resulting resource.
        :param pulumi.Input[str] id: The unique provider ID of the resource to lookup.
        :param pulumi.ResourceOptions opts: Options for the resource.
        """
        opts = pulumi.ResourceOptions.merge(opts, pulumi.ResourceOptions(id=id))

        __props__ = dict()

        return Component(resource_name, opts=opts, __props__=__props__)

    @property
    @pulumi.getter
    def provider(self) -> pulumi.Output[Optional['pulumi_kubernetes.Provider']]:
        return pulumi.get(self, "provider")

    @property
    @pulumi.getter(name="securityGroup")
    def security_group(self) -> pulumi.Output[Optional['pulumi_aws.ec2.SecurityGroup']]:
        return pulumi.get(self, "security_group")

    @property
    @pulumi.getter(name="storageClasses")
    def storage_classes(self) -> pulumi.Output[Optional[Mapping[str, 'pulumi_kubernetes.storage.v1.StorageClass']]]:
        return pulumi.get(self, "storage_classes")

    def translate_output_property(self, prop):
        return _tables.CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop

    def translate_input_property(self, prop):
        return _tables.SNAKE_TO_CAMEL_CASE_TABLE.get(prop) or prop

