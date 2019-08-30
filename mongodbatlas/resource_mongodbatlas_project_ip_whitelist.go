package mongodbatlas

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/spf13/cast"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	matlas "github.com/mongodb/go-client-mongodb-atlas/mongodbatlas"
)

const (
	errorGetInfo = "error getting project IP whitelist information: %s"
)

func resourceMongoDBAtlasProjectIPWhitelist() *schema.Resource {
	return &schema.Resource{
		Create: resourceMongoDBAtlasProjectIPWhitelistCreate,
		Read:   resourceMongoDBAtlasProjectIPWhitelistRead,
		Delete: resourceMongoDBAtlasProjectIPWhitelistDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"project_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"whitelist": {
				Type:     schema.TypeSet,
				Required: true,
				ForceNew: true,
				Set:      filterParamsHash,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"cidr_block": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
							ForceNew: true,
							ValidateFunc: func(i interface{}, k string) (s []string, es []error) {
								v, ok := i.(string)
								if !ok {
									es = append(es, fmt.Errorf("expected type of %s to be string", k))
									return
								}

								_, ipnet, err := net.ParseCIDR(v)
								if err != nil {
									es = append(es, fmt.Errorf(
										"expected %s to contain a valid CIDR, got: %s with err: %s", k, v, err))
									return
								}

								if ipnet == nil || v != ipnet.String() {
									es = append(es, fmt.Errorf(
										"expected %s to contain a valid network CIDR, expected %s, got %s",
										k, ipnet, v))
									return
								}
								return
							},
						},
						"ip_address": {
							Type:         schema.TypeString,
							ForceNew:     true,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.SingleIP(),
						},
						"comment": {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							ValidateFunc: validation.NoZeroValues,
						},
					},
				},
			},
		},
	}
}

func filterParamsHash(v interface{}) int {
	entry := v.(map[string]interface{})
	if cast.ToString(entry["ip_address"]) != "" {
		return hashcode.String(cast.ToString(entry["ip_address"]))
	}
	ip, _, _ := net.ParseCIDR(cast.ToString(entry["cidr_block"]))
	return hashcode.String(ip.String())
}

func resourceMongoDBAtlasProjectIPWhitelistCreate(d *schema.ResourceData, meta interface{}) error {
	//Get the client connection.
	conn := meta.(*matlas.Client)

	projectID := d.Get("project_id").(string)

	req := expandProjectIPWhitelist(d)
	_, _, err := conn.ProjectIPWhitelist.Create(context.Background(), projectID, req)
	if err != nil {
		return fmt.Errorf("error creating project IP whitelist: %s", err)
	}

	var withelist []string
	whiteListMap(req, func(entry string) {
		withelist = append(withelist, entry)
	})

	d.SetId(encodeStateID(map[string]string{
		"project_id": projectID,
		"entries":    strings.Join(withelist, ","),
	}))

	return resourceMongoDBAtlasProjectIPWhitelistRead(d, meta)
}

func resourceMongoDBAtlasProjectIPWhitelistRead(d *schema.ResourceData, meta interface{}) error {
	//Get the client connection.
	conn := meta.(*matlas.Client)
	ids := decodeStateID(d.Id())

	whitelist, err := getProjectIPWhitelist(ids, conn)
	if err != nil {
		return err
	}
	if err := d.Set("whitelist", flattenProjectIPWhitelist(whitelist)); err != nil {
		return fmt.Errorf(errorGetInfo, err)
	}
	return nil
}

func resourceMongoDBAtlasProjectIPWhitelistDelete(d *schema.ResourceData, meta interface{}) error {
	//Get the client connection.
	conn := meta.(*matlas.Client)
	ids := decodeStateID(d.Id())

	whitelist, err := getProjectIPWhitelist(ids, conn)
	if err != nil {
		return err
	}

	whiteListMap(whitelist, func(entry string) {
		_, err = conn.ProjectIPWhitelist.Delete(context.Background(), ids["project_id"], entry)
	})
	if err != nil {
		return fmt.Errorf("error deleting project IP whitelist: %s", err)
	}
	return nil
}

func getProjectIPWhitelist(ids map[string]string, conn *matlas.Client) ([]*matlas.ProjectIPWhitelist, error) {
	projectID := ids["project_id"]
	entries := strings.Split(ids["entries"], ",")

	var whitelist []*matlas.ProjectIPWhitelist
	for _, entry := range entries {
		res, _, err := conn.ProjectIPWhitelist.Get(context.Background(), projectID, entry)
		if err != nil {
			return nil, fmt.Errorf(errorGetInfo, err)
		}
		whitelist = append(whitelist, res)
	}
	return whitelist, nil
}

func whiteListMap(whitelist []*matlas.ProjectIPWhitelist, f func(string)) {
	for _, entry := range whitelist {
		if entry.CIDRBlock != "" {
			f(entry.CIDRBlock)
		} else if entry.IPAddress != "" {
			f(entry.IPAddress)
		}
	}
}

func flattenProjectIPWhitelist(whitelists []*matlas.ProjectIPWhitelist) []map[string]interface{} {
	results := make([]map[string]interface{}, 0)

	for _, whitelist := range whitelists {
		entry := map[string]interface{}{
			"cidr_block": whitelist.CIDRBlock,
			"ip_address": whitelist.IPAddress,
			"comment":    whitelist.Comment,
		}
		results = append(results, entry)
	}
	return results
}

func expandProjectIPWhitelist(d *schema.ResourceData) []*matlas.ProjectIPWhitelist {
	var whitelist []*matlas.ProjectIPWhitelist
	if v, ok := d.GetOk("whitelist"); ok {
		if rs := v.(*schema.Set).List(); len(rs) > 0 {
			whitelist = make([]*matlas.ProjectIPWhitelist, len(rs))
			for k, r := range rs {
				roleMap := r.(map[string]interface{})
				whitelist[k] = &matlas.ProjectIPWhitelist{
					CIDRBlock: roleMap["cidr_block"].(string),
					IPAddress: roleMap["ip_address"].(string),
					Comment:   roleMap["comment"].(string),
				}
			}
		}
	}
	return whitelist
}
